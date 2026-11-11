package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// The setter workspace's Settings tab used to send `visible`, which the API
// ignores for non-admins, so the author saw a switch that did nothing. The
// author's action is now "submit for review" — this proves that transition
// actually lands in the database and that the problem stays private until an
// admin publishes it.
func TestSubmitForReviewThenPublishDB(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	problems := NewProblemStore(db)
	uid := "00000000-0000-0000-0000-000000000000"

	p := &model.Problem{
		ID: uuid.New().String(), Slug: "submit-review-" + uuid.New().String()[:8],
		Title: "Submit Probe", Description: "d", TimeLimit: 1000, MemoryLimit: 262144,
		Difficulty: "easy", Tags: []string{}, CreatedBy: uid,
		Status: model.ProblemStatusDraft,
	}
	if err := problems.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", p.ID)

	// The author submits.
	if err := problems.SetReviewState(ctx, p.ID, model.ProblemStatusPending, "", ""); err != nil {
		t.Fatalf("submit: %v", err)
	}
	got, err := problems.GetByID(ctx, p.ID)
	if err != nil || got == nil {
		t.Fatalf("read back: %v", err)
	}
	if got.Status != model.ProblemStatusPending {
		t.Fatalf("status = %q, want pending — the Settings tab would still show the old state", got.Status)
	}
	// Pending must remain private.
	if got.Visible {
		t.Fatal("a pending problem is visible")
	}

	// An admin publishes it, with a note.
	if err := problems.SetReviewState(ctx, p.ID, model.ProblemStatusPublished, "ok", uid); err != nil {
		t.Fatalf("publish: %v", err)
	}
	got, _ = problems.GetByID(ctx, p.ID)
	if got.Status != model.ProblemStatusPublished || !got.Visible {
		t.Fatalf("after publish: status=%q visible=%v", got.Status, got.Visible)
	}
	if got.ReviewedAt == nil {
		t.Fatal("published problem has no reviewed_at, so the UI cannot show when")
	}

	// Rejection carries the note back to the author.
	if err := problems.SetReviewState(ctx, p.ID, model.ProblemStatusRejected, "test 3 breaks the constraints", uid); err != nil {
		t.Fatalf("reject: %v", err)
	}
	got, _ = problems.GetByID(ctx, p.ID)
	if got.Status != model.ProblemStatusRejected || got.Visible {
		t.Fatalf("after reject: status=%q visible=%v", got.Status, got.Visible)
	}
	if got.ReviewNote == "" {
		t.Fatal("rejection lost its note, which is what the author needs to see")
	}
}
