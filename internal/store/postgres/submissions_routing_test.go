package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// An AI-generated problem is judged locally.
//
// Routing used to be decided by `source != "local"`, so an AI problem
// (source "ai") went to the vjudge submit worker — which asked for a remote
// platform named "ai" that does not exist — while ClaimPending, which requires
// source IN (”,'local'), refused it. The result was a System Error with no
// test-case details and no work done, which is how a perfectly good submission
// came back broken.
func TestAIProblemIsJudgedLocally(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	users := NewUserStore(db)
	problems := NewProblemStore(db)
	subs := NewSubmissionStore(db)

	ownerID := uuid.NewString()
	if err := users.Create(ctx, &model.User{
		ID: ownerID, Username: "routing_" + uuid.NewString()[:8],
		Email: uuid.NewString() + "@aioj.test", PasswordHash: "x", Role: "setter",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", ownerID)

	makeProblem := func(source string) string {
		t.Helper()
		id := uuid.NewString()
		if err := problems.Create(ctx, &model.Problem{
			ID: id, Slug: "routing-" + uuid.NewString()[:8], Title: "Routing",
			Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
			Tags: []string{}, CreatedBy: ownerID, Visible: true, Source: source,
		}); err != nil {
			t.Fatalf("create problem (%s): %v", source, err)
		}
		return id
	}
	aiProblem := makeProblem("ai")
	remoteProblem := makeProblem("codeforces")
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id IN ($1,$2)", aiProblem, remoteProblem)

	makeSubmission := func(problemID string) string {
		t.Helper()
		id := uuid.NewString()
		if err := subs.Create(ctx, &model.Submission{
			ID: id, ProblemID: problemID, UserID: ownerID,
			Language: "cpp-gpp-64", SourceCode: "int main(){}", Status: model.StatusPending,
			SubmissionType: model.SubmissionTypeCode,
		}); err != nil {
			t.Fatalf("create submission: %v", err)
		}
		return id
	}
	aiSub := makeSubmission(aiProblem)
	remoteSub := makeSubmission(remoteProblem)
	defer db.ExecContext(ctx, "DELETE FROM submissions WHERE id IN ($1,$2)", aiSub, remoteSub)

	claimed, err := subs.ClaimPending(ctx, aiSub, uuid.NewString())
	if err != nil {
		t.Fatalf("ClaimPending(ai): %v", err)
	}
	if !claimed {
		t.Error("the local worker must claim a submission on an AI-generated problem")
	}

	claimedRemote, err := subs.ClaimPending(ctx, remoteSub, uuid.NewString())
	if err != nil {
		t.Fatalf("ClaimPending(remote): %v", err)
	}
	if claimedRemote {
		t.Error("a remote-OJ submission must not be judged by the local worker")
	}

	toRemote, err := subs.GetUnsubmittedRemoteSubmissions(ctx)
	if err != nil {
		t.Fatalf("GetUnsubmittedRemoteSubmissions: %v", err)
	}
	seen := map[string]bool{}
	for _, s := range toRemote {
		seen[s.ID] = true
	}
	if seen[aiSub] {
		t.Error("an AI problem's submission must not be submitted to a remote OJ")
	}
	if !seen[remoteSub] {
		t.Error("a remote-OJ problem's submission must be picked up by the submit worker")
	}

	pending, err := subs.ListPending(ctx, 100)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	// aiSub is 'judging' now, so it should not be listed; the remote one must
	// never appear in the local queue.
	for _, id := range pending {
		if id == remoteSub {
			t.Error("a remote-OJ submission must not appear in the local judge queue")
		}
	}
}

// The per-problem rejudge listings follow the same rule: AI problems are
// rejudged locally, so they must be included.
func TestProblemRejudgeIncludesAISource(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	defer db.Close()

	users := NewUserStore(db)
	problems := NewProblemStore(db)
	subs := NewSubmissionStore(db)

	ownerID := uuid.NewString()
	if err := users.Create(ctx, &model.User{
		ID: ownerID, Username: "rejudge_" + uuid.NewString()[:8],
		Email: uuid.NewString() + "@aioj.test", PasswordHash: "x", Role: "setter",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", ownerID)

	problemID := uuid.NewString()
	if err := problems.Create(ctx, &model.Problem{
		ID: problemID, Slug: "rejudge-ai-" + uuid.NewString()[:8], Title: "AI",
		Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
		Tags: []string{}, CreatedBy: ownerID, Visible: true, Source: "ai",
	}); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", problemID)

	subID := uuid.NewString()
	if err := subs.Create(ctx, &model.Submission{
		ID: subID, ProblemID: problemID, UserID: ownerID,
		Language: "cpp-gpp-64", SourceCode: "int main(){}", Status: model.StatusPending,
		SubmissionType: model.SubmissionTypeCode,
	}); err != nil {
		t.Fatalf("create submission: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM submissions WHERE id=$1", subID)

	if err := subs.UpdateResult(ctx, subID, model.StatusAC, 100, 5, 1024, "", nil); err != nil {
		t.Fatalf("UpdateResult: %v", err)
	}

	ids, err := subs.ListIDsForProblemRejudge(ctx, problemID, model.SubmissionFilter{})
	if err != nil {
		t.Fatalf("ListIDsForProblemRejudge: %v", err)
	}
	if len(ids) != 1 || ids[0] != subID {
		t.Fatalf("an AI problem's judged submission must be rejudgeable, got %v", ids)
	}
}
