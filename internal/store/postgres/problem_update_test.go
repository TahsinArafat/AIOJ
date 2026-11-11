package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestProblemStore_UpdateStatement is the regression test for a problem save
// that could never succeed.
//
// Update derived `visible` from `status` but still passed p.Visible as an
// argument, which left $14 referenced nowhere in the statement. Postgres cannot
// infer the type of a parameter that appears nowhere, so every call failed with
// "could not determine data type of parameter $14" and the statement editor
// answered 500 on every save.
func TestProblemStore_UpdateStatement(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	problems := NewProblemStore(db)

	p := &model.Problem{
		ID:          uuid.New().String(),
		Slug:        "upd-" + uuid.New().String()[:8],
		Title:       "Before",
		Description: "old body",
		TimeLimit:   1000,
		MemoryLimit: 262144,
		Difficulty:  "easy",
		Tags:        []string{},
		CreatedBy:   "00000000-0000-0000-0000-000000000000",
		Status:      model.ProblemStatusDraft,
		ScoringMode: "complete",
		ProblemType: model.ProblemTypeStandard,
		CheckerType: "token",
	}
	if err := problems.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", p.ID)

	// The shape the statement editor sends. Visible is included and must be
	// ignored: publishing is an admin decision, not an edit side effect.
	p.Title = "After"
	p.Description = "new body"
	p.Constraints = "1 ≤ n ≤ 10^5"
	p.InputFormat = "One line, n"
	p.OutputFormat = "Answer"
	p.Hint = "Think greedily"
	p.SampleCases = []model.SampleCase{{Input: "3", Output: "3"}}
	p.TimeLimit = 2000
	p.MemoryLimit = 524288
	p.Difficulty = "hard"
	p.Rating = 1500
	p.Tags = []string{"math", "dp"}
	p.CheckerType = "float"
	p.FloatEpsilon = 1e-6
	p.Visible = true

	if err := problems.Update(ctx, p.ID, p); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := problems.GetByID(ctx, p.ID)
	if err != nil || got == nil {
		t.Fatalf("read back: %v", err)
	}
	if got.Title != "After" || got.Description != "new body" {
		t.Fatalf("statement not persisted: title=%q description=%q", got.Title, got.Description)
	}
	if got.Constraints != p.Constraints || got.Hint != p.Hint {
		t.Fatalf("sections not persisted: constraints=%q hint=%q", got.Constraints, got.Hint)
	}
	if got.TimeLimit != 2000 || got.MemoryLimit != 524288 {
		t.Fatalf("limits not persisted: %d/%d", got.TimeLimit, got.MemoryLimit)
	}
	if got.Difficulty != "hard" || got.Rating != 1500 {
		t.Fatalf("difficulty not persisted: %s/%d", got.Difficulty, got.Rating)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "math" || got.Tags[1] != "dp" {
		t.Fatalf("tags not persisted: %v", got.Tags)
	}
	if got.CheckerType != "float" {
		t.Fatalf("checker_type = %q, want %q", got.CheckerType, "float")
	}
	if len(got.SampleCases) != 1 || got.SampleCases[0].Input != "3" {
		t.Fatalf("sample cases not persisted: %+v", got.SampleCases)
	}
	// The reason p.Visible is not an argument.
	if got.Visible {
		t.Fatal("a draft must not become visible through Update")
	}
}

// TestProblemStore_UpdatePublishesWithStatus pins the other half: approval is
// what makes a problem public, so deriving visible from status has to work
// when the status actually changes.
func TestProblemStore_UpdatePublishesWithStatus(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()
	problems := NewProblemStore(db)

	p := &model.Problem{
		ID:          uuid.New().String(),
		Slug:        "pub-" + uuid.New().String()[:8],
		Title:       "Publish me",
		Description: "d",
		TimeLimit:   1000,
		MemoryLimit: 262144,
		Difficulty:  "easy",
		Tags:        []string{},
		CreatedBy:   "00000000-0000-0000-0000-000000000000",
		Status:      model.ProblemStatusDraft,
		ScoringMode: "complete",
		ProblemType: model.ProblemTypeStandard,
		CheckerType: "token",
	}
	if err := problems.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", p.ID)

	p.Status = model.ProblemStatusPublished
	if err := problems.Update(ctx, p.ID, p); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := problems.GetByID(ctx, p.ID)
	if err != nil || got == nil {
		t.Fatalf("read back: %v", err)
	}
	if !got.Visible {
		t.Fatal("a published problem must be visible")
	}
}
