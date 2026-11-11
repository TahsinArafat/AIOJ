package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/contest/format"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/acm"
	"github.com/tahsinarafat/aioj/internal/model"
)

// Manual corrections are a delta layered over the computed standings, never a
// replacement, so the thing that matters is that TotalsByUser sums them per
// participant and that the submission the round was actually scored on is left
// untouched.
func TestContestAdjustmentsSumPerUser(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	users := NewUserStore(db)
	contests := NewContestStore(db)
	store := NewContestAdjustmentStore(db)

	owner := &model.User{
		ID: uuid.New().String(), Username: "adj_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "hashed", Role: "setter",
	}
	if err := users.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	contest := &model.Contest{
		ID: uuid.New().String(), Title: "Adj Cup", Type: "acm", Format: "acm",
		CreatedBy: owner.ID, Status: model.ContestStatusDraft,
	}
	if err := contests.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", contest.ID)

	alice := &model.User{
		ID: uuid.New().String(), Username: "alice_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "hashed", Role: "user",
	}
	bob := &model.User{
		ID: uuid.New().String(), Username: "bob_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "hashed", Role: "user",
	}
	for _, u := range []*model.User{alice, bob} {
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("create %s: %v", u.Username, err)
		}
		defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)
	}
	defer db.ExecContext(ctx, "DELETE FROM contest_adjustments WHERE contest_id = $1", contest.ID)

	mk := func(user string, points, solved, penalty int, reason string) {
		a := &model.ContestAdjustment{
			ContestID: contest.ID, UserID: user, Scope: model.AdjustmentContest,
			PointDelta: points, SolvedDelta: solved, PenaltyDelta: penalty,
			Reason: reason, CreatedBy: &owner.ID,
		}
		if err := store.Create(ctx, a); err != nil {
			t.Fatalf("create adjustment: %v", err)
		}
		if a.CreatedAt.IsZero() {
			t.Fatal("created_at should be stamped by the store")
		}
	}

	// Alice: a broken test case credited her a problem, then a second ruling
	// added time back.
	mk(alice.ID, 100, 1, 0, "test case 4 was wrong")
	mk(alice.ID, 0, 0, 120, "judge outage, time restored")
	// Bob: a penalty for a clarification violation.
	mk(bob.ID, 0, 0, 300, "submitted after clarification ruling")

	totals, err := store.TotalsByUser(ctx, contest.ID)
	if err != nil {
		t.Fatalf("TotalsByUser: %v", err)
	}

	a := totals[alice.ID]
	if a.PointDelta != 100 || a.SolvedDelta != 1 || a.PenaltyDelta != 120 {
		t.Fatalf("alice totals = %+v, want +100 pts, +1 solved, +120 penalty", a)
	}
	if a.IsZero() {
		t.Fatal("alice's totals should not be zero")
	}

	b := totals[bob.ID]
	if b.PointDelta != 0 || b.SolvedDelta != 0 || b.PenaltyDelta != 300 {
		t.Fatalf("bob totals = %+v, want only +300 penalty", b)
	}

	// A participant with no corrections has no entry, so the scoreboard's lookup
	// finds nothing and applies nothing.
	if _, ok := totals[owner.ID]; ok {
		t.Fatal("a user with no corrections must not appear in the totals")
	}
}

// The history has to be readable months later: it names the person, the team
// they competed as, and the problem letter they are about.
func TestContestAdjustmentHistoryNamesThePerson(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	users := NewUserStore(db)
	problems := NewProblemStore(db)
	contests := NewContestStore(db)
	store := NewContestAdjustmentStore(db)

	owner := &model.User{
		ID: uuid.New().String(), Username: "hst_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "hashed", Role: "setter",
	}
	if err := users.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	contest := &model.Contest{
		ID: uuid.New().String(), Title: "Hist Cup", Type: "acm", Format: "acm",
		CreatedBy: owner.ID, Status: model.ContestStatusDraft,
	}
	if err := contests.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", contest.ID)

	problem := &model.Problem{
		ID: uuid.New().String(), Slug: "hst-" + uuid.New().String()[:8], Title: "Sum",
		Description: "d", TimeLimit: 1000, MemoryLimit: 262144, Difficulty: "easy",
		Tags: []string{}, CreatedBy: owner.ID, Visible: true,
	}
	if err := problems.Create(ctx, problem); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", problem.ID)
	if err := contests.AddProblem(ctx, contest.ID, problem.ID, "A", 100, 0); err != nil {
		t.Fatalf("add problem: %v", err)
	}

	team := &model.User{
		ID: uuid.New().String(), Username: "team_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@onsite.aioj", PasswordHash: "hashed",
		Role: model.RoleContestant,
	}
	if err := users.Create(ctx, team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", team.ID)
	if _, err := db.ExecContext(ctx,
		`INSERT INTO onsite_batch_users (id, contest_id, team_name, institution, username, password_hash, is_used, used_by)
		 VALUES ($1,$2,$3,$4,$5,$6,true,$7)`,
		uuid.New().String(), contest.ID, "Team Rocket", "MIT", team.Username, "hashed", team.ID,
	); err != nil {
		t.Fatalf("insert onsite team: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contest_adjustments WHERE contest_id = $1", contest.ID)

	pid := problem.ID
	a := &model.ContestAdjustment{
		ContestID: contest.ID, UserID: team.ID, Scope: model.AdjustmentProblem,
		ProblemID: &pid, PointDelta: 50, Reason: "ambiguous statement",
		CreatedBy: &owner.ID,
	}
	if err := store.Create(ctx, a); err != nil {
		t.Fatalf("create adjustment: %v", err)
	}

	items, err := store.ListByContest(ctx, contest.ID)
	if err != nil {
		t.Fatalf("ListByContest: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 adjustment, got %d", len(items))
	}
	got := items[0]
	// The team name is what a judge recognises, and the letter is how the round
	// is read.
	if got.TeamName != "Team Rocket" {
		t.Fatalf("team_name = %q, want Team Rocket", got.TeamName)
	}
	if got.ProblemIndex == nil || *got.ProblemIndex != "A" {
		t.Fatalf("problem_index = %v, want A", got.ProblemIndex)
	}
	if got.Reason != "ambiguous statement" {
		t.Fatalf("reason = %q", got.Reason)
	}
}

// A zero adjustment is a no-op that would sit in the history looking like work.
// The schema refuses it, which is what keeps the ledger honest.
func TestContestAdjustmentRejectsNoOp(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	users := NewUserStore(db)
	contests := NewContestStore(db)
	store := NewContestAdjustmentStore(db)

	owner := &model.User{
		ID: uuid.New().String(), Username: "nop_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "hashed", Role: "setter",
	}
	if err := users.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	contest := &model.Contest{
		ID: uuid.New().String(), Title: "Noop", Type: "acm", Format: "acm",
		CreatedBy: owner.ID, Status: model.ContestStatusDraft,
	}
	if err := contests.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", contest.ID)

	err := store.Create(ctx, &model.ContestAdjustment{
		ContestID: contest.ID, UserID: owner.ID, Scope: model.AdjustmentContest,
		Reason: "nothing at all",
	})
	if err == nil {
		t.Fatal("a correction with no deltas must be refused by the schema")
	}
}

// A correction exists to change the outcome, so the thing worth pinning is that
// it reorders the board — not merely that a column was written.
//
// This mirrors what ContestHandler.Scoreboard does: score the submissions, apply
// the deltas, and rank the result. Applying deltas after ranking would leave the
// order untouched and the correction cosmetic.
func TestAdjustmentChangesRankOrder(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	ctx := context.Background()

	users := NewUserStore(db)
	contests := NewContestStore(db)
	store := NewContestAdjustmentStore(db)

	owner := &model.User{ID: uuid.New().String(), Username: "e2e_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "h", Role: "setter"}
	if err := users.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	contest := &model.Contest{ID: uuid.New().String(), Title: "E2E", Type: "acm", Format: "acm",
		CreatedBy: owner.ID, Status: model.ContestStatusDraft}
	if err := contests.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", contest.ID)
	defer db.ExecContext(ctx, "DELETE FROM contest_adjustments WHERE contest_id = $1", contest.ID)

	alice := &model.User{ID: uuid.New().String(), Username: "ea_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "h", Role: "user"}
	bob := &model.User{ID: uuid.New().String(), Username: "eb_" + uuid.New().String()[:8],
		Email: uuid.New().String()[:8] + "@ex.com", PasswordHash: "h", Role: "user"}
	for _, u := range []*model.User{alice, bob} {
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("create %s: %v", u.Username, err)
		}
		defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)
	}

	f, err := format.Create("acm", nil)
	if err != nil {
		t.Fatalf("create format: %v", err)
	}

	// Bob solved one problem; Alice none, so Bob leads before the correction.
	build := func() []format.ParticipantScore {
		ps := []format.ParticipantScore{
			{UserID: alice.ID, Username: alice.Username},
			{UserID: bob.ID, Username: bob.Username, TotalSolved: 1, TotalScore: 1, TotalPenalty: 30},
		}
		totals, err := store.TotalsByUser(ctx, contest.ID)
		if err != nil {
			t.Fatalf("TotalsByUser: %v", err)
		}
		for i := range ps {
			if adj, ok := totals[ps[i].UserID]; ok {
				ps[i].TotalScore += float64(adj.PointDelta)
				ps[i].TotalSolved += adj.SolvedDelta
				ps[i].TotalPenalty += adj.PenaltyDelta
			}
		}
		return ps
	}

	if before := f.RankParticipants(build()); before[0].Score.UserID != bob.ID {
		t.Fatalf("precondition: Bob should lead, got %s", before[0].Score.UserID)
	}

	// A judge rules a broken test case cost Alice a problem.
	if err := store.Create(ctx, &model.ContestAdjustment{
		ContestID: contest.ID, UserID: alice.ID, Scope: model.AdjustmentContest,
		PointDelta: 1, SolvedDelta: 1,
		Reason: "test case 4 was wrong", CreatedBy: &owner.ID,
	}); err != nil {
		t.Fatalf("create adjustment: %v", err)
	}

	after := f.RankParticipants(build())
	if after[0].Score.UserID != alice.ID {
		t.Fatalf("the correction did not move Alice to first: leader is %s", after[0].Score.UserID)
	}
	if after[0].Score.TotalSolved != 1 {
		t.Fatalf("alice solved = %d, want 1 after the correction", after[0].Score.TotalSolved)
	}
}
