package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// A problem's type must survive a create/read round trip, because the submission
// path keys off it: get it wrong and an output-only problem would demand a
// language, or a coded problem would be judged as literal stdout with no compile.
func TestProblemTypeRoundTripsDB(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	problems := NewProblemStore(db)
	uid := "00000000-0000-0000-0000-000000000000"

	cases := []string{model.ProblemTypeStandard, model.ProblemTypeOutputOnly, model.ProblemTypeInteractive}
	for _, want := range cases {
		t.Run(want, func(t *testing.T) {
			p := &model.Problem{
				ID: uuid.New().String(), Slug: "ptype-" + uuid.New().String()[:8],
				Title: "Type Probe", Description: "d", TimeLimit: 1000, MemoryLimit: 262144,
				Difficulty: "easy", Tags: []string{}, CreatedBy: uid,
				Status: model.ProblemStatusDraft, ProblemType: want,
			}
			if err := problems.Create(ctx, p); err != nil {
				t.Fatalf("create: %v", err)
			}
			defer db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", p.ID)

			got, err := problems.GetByID(ctx, p.ID)
			if err != nil || got == nil {
				t.Fatalf("read back: %v", err)
			}
			if got.ProblemType != want {
				t.Fatalf("problem_type = %q, want %q", got.ProblemType, want)
			}
		})
	}
}

// An empty type must default rather than violate the CHECK constraint: the AI
// apply path and the seeder predate this column.
func TestProblemTypeDefaultsToStandardDB(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	problems := NewProblemStore(db)
	uid := "00000000-0000-0000-0000-000000000000"

	p := &model.Problem{
		ID: uuid.New().String(), Slug: "ptype-default-" + uuid.New().String()[:8],
		Title: "Default Probe", Description: "d", TimeLimit: 1000, MemoryLimit: 262144,
		Difficulty: "easy", Tags: []string{}, CreatedBy: uid,
		Status: model.ProblemStatusDraft,
		// ProblemType deliberately empty.
	}
	if err := problems.Create(ctx, p); err != nil {
		t.Fatalf("create with empty type: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id=$1", p.ID)

	got, _ := problems.GetByID(ctx, p.ID)
	if got.ProblemType != model.ProblemTypeStandard {
		t.Fatalf("problem_type = %q, want standard", got.ProblemType)
	}
}
