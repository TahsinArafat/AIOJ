package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestBalloonListLabelsOnsiteAccounts is the regression test for a runner
// sheet that printed "team_1a2b3c4d" instead of the team to deliver to.
//
// A balloon is handed to a person, so the name matters more here than almost
// anywhere else. A generated onsite account's username is a throwaway handle;
// the team name exists only in onsite_batch_users, which this list never
// joined.
func TestBalloonListLabelsOnsiteAccounts(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	probStore := NewProblemStore(db)
	contestStore := NewContestStore(db)
	subStore := NewSubmissionStore(db)
	balloonStore := NewBalloonStore(db)

	owner := &model.User{
		ID:           uuid.New().String(),
		Username:     "bl_" + uuid.New().String()[:8],
		Email:        "bl_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed",
		Role:         "setter",
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	contest := &model.Contest{
		ID:        uuid.New().String(),
		Title:     "Balloon Cup",
		Type:      "acm",
		Format:    "acm",
		CreatedBy: owner.ID,
		Status:    model.ContestStatusDraft,
	}
	if err := contestStore.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", contest.ID)

	problem := &model.Problem{
		ID:          uuid.New().String(),
		Slug:        "bl-prob-" + uuid.New().String()[:8],
		Title:       "A+B",
		Description: "sum",
		TimeLimit:   1000,
		MemoryLimit: 262144,
		Difficulty:  "easy",
		Tags:        []string{},
		CreatedBy:   owner.ID,
		Visible:     true,
	}
	if err := probStore.Create(ctx, problem); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", problem.ID)

	if err := contestStore.AddProblem(ctx, contest.ID, problem.ID, "A", 100, 0); err != nil {
		t.Fatalf("add problem to contest: %v", err)
	}

	// The generated account, as the Onsite Teams panel mints it.
	teamUser := &model.User{
		ID:           uuid.New().String(),
		Username:     "team_" + uuid.New().String()[:8],
		Email:        uuid.New().String()[:8] + "@onsite.aioj",
		PasswordHash: "hashed",
		Role:         model.RoleContestant,
	}
	if err := userStore.Create(ctx, teamUser); err != nil {
		t.Fatalf("create team user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", teamUser.ID)

	if _, err := db.ExecContext(ctx,
		`INSERT INTO onsite_batch_users (id, contest_id, team_name, institution, username, password_hash, is_used, used_by)
		 VALUES ($1,$2,$3,$4,$5,$6,true,$7)`,
		uuid.New().String(), contest.ID, "Team Rocket", "MIT", teamUser.Username, "hashed", teamUser.ID,
	); err != nil {
		t.Fatalf("insert onsite team: %v", err)
	}

	sub := &model.Submission{
		ID:             uuid.New().String(),
		ProblemID:      problem.ID,
		UserID:         teamUser.ID,
		ContestID:      contest.ID,
		Language:       "cpp",
		SourceCode:     "int main() {}",
		Status:         model.StatusAC,
		SubmissionType: model.SubmissionTypeCode,
	}
	if err := subStore.Create(ctx, sub); err != nil {
		t.Fatalf("create submission: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM submissions WHERE id = $1", sub.ID)

	if err := balloonStore.CreateRequest(ctx, contest.ID, sub.ID, teamUser.ID, problem.ID); err != nil {
		t.Fatalf("create balloon request: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM balloon_requests WHERE contest_id = $1", contest.ID)

	list, err := balloonStore.ListByContest(ctx, contest.ID)
	if err != nil {
		t.Fatalf("ListByContest: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 balloon, got %d", len(list))
	}

	got := list[0]
	if got.TeamName != "Team Rocket" || got.Institution != "MIT" {
		t.Fatalf("balloon label = %+v, want Team Rocket / MIT", got)
	}
	// The username stays on the row: it is what the account signs in with.
	if got.Username != teamUser.Username {
		t.Fatalf("username = %q, want %q", got.Username, teamUser.Username)
	}
}
