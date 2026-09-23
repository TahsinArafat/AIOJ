package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestSubmissionClaimFencesStaleWorker proves the public store contract used
// by multiple judge workers: only one worker owns a pending submission, a
// stale owner can be reclaimed, and only the current claim may write a verdict.
func TestSubmissionClaimFencesStaleWorker(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	users := NewUserStore(db)
	problems := NewProblemStore(db)
	submissions := NewSubmissionStore(db)

	userID := uuid.NewString()
	user := &model.User{
		ID: userID, Username: "recovery_" + uuid.NewString()[:8],
		Email: uuid.NewString() + "@aioj.test", PasswordHash: "hash", Role: "user",
	}
	if err := users.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", userID)

	problemID := uuid.NewString()
	problem := &model.Problem{
		ID: problemID, Slug: "recovery-" + uuid.NewString()[:8], Title: "Recovery",
		Description: "worker recovery", TimeLimit: 1000, MemoryLimit: 262144,
		Difficulty: "easy", Tags: []string{}, CreatedBy: userID, Visible: true,
	}
	if err := problems.Create(ctx, problem); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", problemID)

	submissionID := uuid.NewString()
	if err := submissions.Create(ctx, &model.Submission{
		ID: submissionID, ProblemID: problemID, UserID: userID,
		Language: "cpp-gpp-64", SourceCode: "int main() {}", Status: model.StatusPending,
		SubmissionType: model.SubmissionTypeCode,
	}); err != nil {
		t.Fatalf("create submission: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM submissions WHERE id=$1", submissionID)

	firstClaim := uuid.NewString()
	claimed, err := submissions.ClaimPending(ctx, submissionID, firstClaim)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if !claimed {
		t.Fatal("first worker should claim the pending submission")
	}

	claimed, err = submissions.ClaimPending(ctx, submissionID, uuid.NewString())
	if err != nil {
		t.Fatalf("competing claim: %v", err)
	}
	if claimed {
		t.Fatal("a second worker must not claim an already-judging submission")
	}

	if _, err := db.ExecContext(ctx,
		"UPDATE submissions SET judging_started_at=NOW()-INTERVAL '1 hour' WHERE id=$1",
		submissionID); err != nil {
		t.Fatalf("age claim: %v", err)
	}
	ids, err := submissions.RequeueStale(ctx, 15*time.Minute)
	if err != nil {
		t.Fatalf("requeue stale: %v", err)
	}
	if len(ids) != 1 || ids[0] != submissionID {
		t.Fatalf("requeued ids = %v, want [%s]", ids, submissionID)
	}

	secondClaim := uuid.NewString()
	claimed, err = submissions.ClaimPending(ctx, submissionID, secondClaim)
	if err != nil {
		t.Fatalf("second-generation claim: %v", err)
	}
	if !claimed {
		t.Fatal("reclaimed submission should be claimable by the next worker")
	}

	applied, err := submissions.UpdateResultClaimed(ctx, submissionID, firstClaim,
		model.StatusWA, 0, 1, 1, "stale worker", nil)
	if err != nil {
		t.Fatalf("stale result update: %v", err)
	}
	if applied {
		t.Fatal("the stale worker must not overwrite the current claim")
	}

	applied, err = submissions.UpdateResultClaimed(ctx, submissionID, secondClaim,
		model.StatusAC, 100, 2, 2, "", nil)
	if err != nil {
		t.Fatalf("current result update: %v", err)
	}
	if !applied {
		t.Fatal("the current claim should persist its verdict")
	}

	got, err := submissions.GetByID(ctx, submissionID)
	if err != nil || got == nil {
		t.Fatalf("read submission: submission=%v err=%v", got, err)
	}
	if got.Status != model.StatusAC {
		t.Fatalf("status = %s, want ac", got.Status)
	}
}
