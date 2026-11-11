package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// A contest's problem letters are a function of position, not a fact stored once
// and left alone.
//
// The letter lived in contest_problems.index and was never revisited when a
// problem was removed, so a deletion left a permanent hole. Half the UI then
// papered over the hole with its own `'A' + position` while the other half read
// the stored value — which is how the problems tab came to show "B" for the
// problem the standings called "A".
//
// RenumberProblems is the single repair, and these tests pin the two properties
// that matter: the letters come out gapless, and the order they are assigned in
// is the order a reader sees.
func TestRenumberProblemsClosesTheGap(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	users := NewUserStore(db)
	problems := NewProblemStore(db)
	contests := NewContestStore(db)

	owner := &model.User{
		ID: uuid.New().String(), Username: "rw_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "hashed", Role: "setter",
	}
	if err := users.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	contest := &model.Contest{
		ID: uuid.New().String(), Title: "Renumber Cup", Type: "acm", Format: "acm",
		CreatedBy: owner.ID, Status: model.ContestStatusDraft,
	}
	if err := contests.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", contest.ID)

	mkProblem := func(tag string) string {
		id := uuid.New().String()
		p := &model.Problem{
			ID: id, Slug: "rw-" + tag + "-" + uuid.New().String()[:8], Title: tag,
			Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
			Tags: []string{}, CreatedBy: owner.ID, Visible: true,
		}
		if err := problems.Create(ctx, p); err != nil {
			t.Fatalf("create problem %s: %v", tag, err)
		}
		return id
	}
	a, b, c := mkProblem("A"), mkProblem("B"), mkProblem("C")
	for _, id := range []string{a, b, c} {
		defer db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", id)
	}

	// A round as it was originally set: A, B, C.
	for i, id := range []string{a, b, c} {
		if err := contests.AddProblem(ctx, contest.ID, id, model.ContestProblemIndex(i), 100, i); err != nil {
			t.Fatalf("add problem: %v", err)
		}
	}
	if err := contests.RemoveProblem(ctx, contest.ID, b); err != nil {
		t.Fatalf("remove problem: %v", err)
	}

	before, _ := contests.GetProblems(ctx, contest.ID)
	if len(before) != 2 {
		t.Fatalf("want 2 problems after removal, got %d", len(before))
	}
	// The gap this test exists for: A survived, then C, with no B.
	if before[0].Index != "A" || before[1].Index != "C" {
		t.Fatalf("precondition: stored letters = %q, %q; want a gap at B",
			before[0].Index, before[1].Index)
	}

	after, err := contests.RenumberProblems(ctx, contest.ID)
	if err != nil {
		t.Fatalf("RenumberProblems: %v", err)
	}

	// Gapless, in order, and sort_order rewritten to match.
	want := []string{"A", "B"}
	if len(after) != len(want) {
		t.Fatalf("want %d problems, got %d", len(want), len(after))
	}
	for i, w := range want {
		if after[i].Index != w {
			t.Fatalf("position %d = %q, want %q", i, after[i].Index, w)
		}
		if after[i].SortOrder != i {
			t.Fatalf("position %d sort_order = %d, want %d", i, after[i].SortOrder, i)
		}
		if after[i].ProblemID != []string{a, c}[i] {
			t.Fatalf("position %d is the wrong problem: %s", i, after[i].ProblemID)
		}
	}

	// And it is what the store now serves, not just what the call returned.
	persisted, _ := contests.GetProblems(ctx, contest.ID)
	for i := range want {
		if persisted[i].Index != want[i] {
			t.Fatalf("persisted position %d = %q, want %q", i, persisted[i].Index, want[i])
		}
	}
}

// A round whose sort_order was never maintained must still come out in a
// sensible order: the existing index is the tie-break, so the result is
// deterministic rather than whatever the table scan produced.
func TestRenumberProblemsBreaksTiesOnExistingIndex(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	users := NewUserStore(db)
	problems := NewProblemStore(db)
	contests := NewContestStore(db)

	owner := &model.User{
		ID: uuid.New().String(), Username: "rt_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "hashed", Role: "setter",
	}
	if err := users.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	contest := &model.Contest{
		ID: uuid.New().String(), Title: "Tie Cup", Type: "acm", Format: "acm",
		CreatedBy: owner.ID, Status: model.ContestStatusDraft,
	}
	if err := contests.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", contest.ID)

	// Two problems, both sort_order 0 — the state a round is left in by an
	// add path that never set an order.
	var ids []string
	for _, tag := range []string{"P", "Q"} {
		id := uuid.New().String()
		p := &model.Problem{
			ID: id, Slug: "rt-" + tag + "-" + uuid.New().String()[:8], Title: tag,
			Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
			Tags: []string{}, CreatedBy: owner.ID, Visible: true,
		}
		if err := problems.Create(ctx, p); err != nil {
			t.Fatalf("create problem: %v", err)
		}
		defer db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", id)
		ids = append(ids, id)
	}
	// Deliberately stored out of index order to prove the index is the tie-break.
	if err := contests.AddProblem(ctx, contest.ID, ids[0], "Q", 100, 0); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := contests.AddProblem(ctx, contest.ID, ids[1], "P", 100, 0); err != nil {
		t.Fatalf("add: %v", err)
	}

	after, err := contests.RenumberProblems(ctx, contest.ID)
	if err != nil {
		t.Fatalf("RenumberProblems: %v", err)
	}
	if len(after) != 2 || after[0].Index != "A" || after[1].Index != "B" {
		t.Fatalf("letters = %+v, want A then B", after)
	}
	// "P" sorted before "Q", so it takes A.
	if after[0].ProblemID != ids[1] {
		t.Fatalf("the index tie-break was not honoured: %+v", after)
	}
}
