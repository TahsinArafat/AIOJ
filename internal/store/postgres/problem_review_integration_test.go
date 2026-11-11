package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// The review workflow end to end against a real database: a setter's problem is
// invisible to the archive until an admin publishes it, and unpublishing takes it
// back out. This is the behaviour the CHECK constraints and the archive predicate
// are supposed to guarantee together, and only a real Postgres can prove the two
// agree.
func TestProblemReviewWorkflowDB(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	userStore := NewUserStore(db)
	problems := NewProblemStore(db)

	userID := uuid.New().String()
	if err := userStore.Create(ctx, &model.User{
		ID: userID, Username: "review_" + uuid.New().String()[:8],
		Email:        "review_" + uuid.New().String()[:8] + "@aioj.test",
		PasswordHash: "x", Role: model.RoleSetter,
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id=$1", userID)

	p := &model.Problem{
		ID: uuid.New().String(), Slug: "review-" + uuid.New().String()[:8],
		Title: "Review Workflow", Description: "d", TimeLimit: 1000, MemoryLimit: 262144,
		Difficulty: "easy", Tags: []string{}, CreatedBy: userID,
		Status: model.ProblemStatusDraft,
	}
	if err := problems.Create(ctx, p); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", p.ID)

	// A draft must not be visible, whatever the caller asked for.
	stored, err := problems.GetByID(ctx, p.ID)
	if err != nil || stored == nil {
		t.Fatalf("read back: %v", err)
	}
	if stored.Visible {
		t.Fatal("a draft problem was created visible")
	}
	if inArchive(t, db, p.ID) {
		t.Fatal("a draft problem appeared in the public archive")
	}

	// Submitting moves it to pending, still not public.
	if err := problems.SetReviewState(ctx, p.ID, model.ProblemStatusPending, "", ""); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if inArchive(t, db, p.ID) {
		t.Fatal("a pending problem appeared in the public archive")
	}

	// Approval is what publishes it.
	if err := problems.SetReviewState(ctx, p.ID, model.ProblemStatusPublished, "ok", userID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	stored, _ = problems.GetByID(ctx, p.ID)
	if !stored.Visible {
		t.Fatal("publishing did not make the problem visible")
	}
	if !inArchive(t, db, p.ID) {
		t.Fatal("a published problem is missing from the public archive")
	}

	// Rejection takes it back out.
	if err := problems.SetReviewState(ctx, p.ID, model.ProblemStatusRejected, "no", userID); err != nil {
		t.Fatalf("reject: %v", err)
	}
	stored, _ = problems.GetByID(ctx, p.ID)
	if stored.Visible {
		t.Fatal("a rejected problem is still visible")
	}
	if inArchive(t, db, p.ID) {
		t.Fatal("a rejected problem is still in the public archive")
	}
}

// inArchive asks the same predicate the public list uses, so this test fails if
// the archive ever stops agreeing with the review state.
func inArchive(t *testing.T, db *sql.DB, id string) bool {
	t.Helper()
	var found bool
	if err := db.QueryRowContext(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM problems p WHERE p.id=$1 AND "+archivedProblemSQL+")",
		id).Scan(&found); err != nil {
		t.Fatalf("archive probe: %v", err)
	}
	return found
}
