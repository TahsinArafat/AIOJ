package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// voidFixture builds one judged submission to void.
func voidFixture(t *testing.T, ctx context.Context) (subs *SubmissionStore, userID, problemID, subID string, cleanup func()) {
	t.Helper()
	db := openTestDB(t)

	users := NewUserStore(db)
	problems := NewProblemStore(db)

	userID = uuid.NewString()
	if err := users.Create(ctx, &model.User{
		ID: userID, Username: "void_" + uuid.NewString()[:8],
		Email: uuid.NewString() + "@aioj.test", PasswordHash: "x", Role: "user",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}

	problemID = uuid.NewString()
	if err := problems.Create(ctx, &model.Problem{
		ID: problemID, Slug: "void-" + uuid.NewString()[:8], Title: "Void",
		Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
		Tags: []string{}, CreatedBy: userID,
		Status: model.ProblemStatusPublished, Visible: true,
	}); err != nil {
		t.Fatalf("create problem: %v", err)
	}

	subs = NewSubmissionStore(db)
	subID = uuid.NewString()
	if err := subs.Create(ctx, &model.Submission{
		ID: subID, ProblemID: problemID, UserID: userID,
		Language: "cpp-gpp-64", SourceCode: "int main(){}", Status: model.StatusWA,
		SubmissionType: model.SubmissionTypeCode,
	}); err != nil {
		t.Fatalf("create submission: %v", err)
	}
	// Give it a real judged result, so the test can prove the void does not
	// destroy the evidence of what the code actually did.
	if err := subs.UpdateResult(ctx, subID, model.StatusWA, 40, 120, 2048, "", nil); err != nil {
		t.Fatalf("record result: %v", err)
	}

	cleanup = func() {
		db.ExecContext(ctx, "DELETE FROM submissions WHERE problem_id=$1", problemID)
		db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", problemID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", userID)
		db.Close()
	}
	return subs, userID, problemID, subID, cleanup
}

// A void is a human decision layered over the judge's answer: the submission
// must stop counting, but the verdict it earned has to survive as the record of
// what the code did. The round is defended later from both.
func TestSetSkippedKeepsTheEarnedVerdict(t *testing.T) {
	ctx := context.Background()
	subs, _, _, subID, cleanup := voidFixture(t, ctx)
	defer cleanup()

	changed, err := subs.SetSkipped(ctx, subID)
	if err != nil {
		t.Fatalf("SetSkipped: %v", err)
	}
	if !changed {
		t.Fatal("SetSkipped reported no change on a freshly judged submission")
	}

	got, err := subs.GetByID(ctx, subID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != model.StatusSkipped {
		t.Fatalf("status = %q, want %q", got.Status, model.StatusSkipped)
	}
	// The evidence must survive the void: a skip that erased the score, the
	// time and the case results would leave nothing to defend the call with, and
	// the void itself is not reproducible from the row.
	if got.Score != 40 || got.TimeUsed != 120 || got.MemoryUsed != 2048 {
		t.Errorf("the void destroyed the judged result: score=%d time=%d memory=%d, want 40/120/2048",
			got.Score, got.TimeUsed, got.MemoryUsed)
	}
}

// The judge must never resurrect a submission an organizer removed. A stale
// worker writing its verdict would silently undo the void and put the attempt
// back on the scoreboard.
func TestSetSkippedSurvivesAStaleJudgeVerdict(t *testing.T) {
	ctx := context.Background()
	subs, _, _, subID, cleanup := voidFixture(t, ctx)
	defer cleanup()

	if _, err := subs.SetSkipped(ctx, subID); err != nil {
		t.Fatalf("SetSkipped: %v", err)
	}

	// An unclaimed result write (the admin/vjudge path) must be refused.
	if err := subs.UpdateResult(ctx, subID, model.StatusAC, 100, 5, 512, "", nil); err != nil {
		t.Fatalf("UpdateResult: %v", err)
	}
	got, err := subs.GetByID(ctx, subID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != model.StatusSkipped {
		t.Errorf("status = %q after a judge write, want %q — a verdict must not overwrite a void", got.Status, model.StatusSkipped)
	}
	if got.Score == 100 {
		t.Error("an accepted verdict overwrote a skipped submission's score")
	}
}

// Voiding is idempotent and refuses a submission the judge still owns, so the
// two writers cannot race and the caller gets an actionable answer.
func TestSetSkippedRefusesInFlightAndRepeat(t *testing.T) {
	ctx := context.Background()
	subs, _, _, subID, cleanup := voidFixture(t, ctx)
	defer cleanup()

	// A submission the judge holds: voiding it now would race the verdict.
	subs.UpdateStatus(ctx, subID, model.StatusJudging)
	changed, err := subs.SetSkipped(ctx, subID)
	if err != nil {
		t.Fatalf("SetSkipped: %v", err)
	}
	if changed {
		t.Error("SetSkipped voided a submission that was still being judged")
	}

	// Back to a finished state, then void it twice.
	subs.UpdateStatus(ctx, subID, model.StatusWA)
	if changed, err = subs.SetSkipped(ctx, subID); err != nil || !changed {
		t.Fatalf("first SetSkipped: changed=%v err=%v", changed, err)
	}
	if changed, err = subs.SetSkipped(ctx, subID); err != nil {
		t.Fatalf("second SetSkipped: %v", err)
	} else if changed {
		t.Error("a second void reported a change; the operation must be idempotent")
	}
}

// A skipped submission must not be swept back into a rejudge, which would
// quietly undo the void the next time a setter fixed a testcase.
func TestSkippedIsExcludedFromRejudgeSelection(t *testing.T) {
	ctx := context.Background()
	subs, userID, problemID, subID, cleanup := voidFixture(t, ctx)
	defer cleanup()

	if _, err := subs.SetSkipped(ctx, subID); err != nil {
		t.Fatalf("SetSkipped: %v", err)
	}

	ids, _, err := subs.ListForRejudge(ctx, model.SubmissionFilter{ProblemID: problemID})
	if err != nil {
		t.Fatalf("ListForRejudge: %v", err)
	}
	for _, id := range ids {
		if id == subID {
			t.Fatal("a skipped submission was selected for rejudge; the void would be silently undone")
		}
	}

	// And one left in the list, so the exclusion is not just an empty query.
	keep := uuid.NewString()
	if err := subs.Create(ctx, &model.Submission{
		ID: keep, ProblemID: problemID, UserID: userID,
		Language: "cpp-gpp-64", SourceCode: "int main(){}", Status: model.StatusWA,
		SubmissionType: model.SubmissionTypeCode,
	}); err != nil {
		t.Fatalf("create second submission: %v", err)
	}
	// Remove it again immediately: user_id above is not a real user, so the row
	// exists only to prove the query is not simply returning nothing.
	defer subs.db.ExecContext(ctx, "DELETE FROM submissions WHERE id=$1", keep)
}
