package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestGetDisplayNamesLabelsOnsiteAccounts is the regression test for standings
// that showed "team_1a2b3c4d" instead of the team the account was minted for.
//
// The competitor knows themselves as the team name entered in the Onsite Teams
// panel, which lives only in onsite_batch_users. The scoreboard resolved names
// from users.username alone, so a generated account had no way to show its team.
func TestGetDisplayNamesLabelsOnsiteAccounts(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	contestStore := NewContestStore(db)
	userStore := NewUserStore(db)

	owner := &model.User{
		ID:           uuid.New().String(),
		Username:     "dn_" + uuid.New().String()[:8],
		Email:        "dn_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed",
		Role:         "setter",
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	contest := &model.Contest{
		ID:        uuid.New().String(),
		Title:     "Onsite",
		Type:      "acm",
		Format:    "acm",
		CreatedBy: owner.ID,
		Status:    model.ContestStatusDraft,
	}
	if err := contestStore.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", contest.ID)

	// A generated account, exactly as the Onsite Teams panel mints it, plus a
	// row in onsite_batch_users carrying the team name.
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

	// An ordinary user in the same contest must fall back to their username.
	plain := &model.User{
		ID:           uuid.New().String(),
		Username:     "solo_" + uuid.New().String()[:8],
		Email:        uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed",
		Role:         model.RoleUser,
	}
	if err := userStore.Create(ctx, plain); err != nil {
		t.Fatalf("create plain user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", plain.ID)

	names, err := contestStore.GetDisplayNames(ctx, contest.ID, []string{teamUser.ID, plain.ID})
	if err != nil {
		t.Fatalf("GetDisplayNames: %v", err)
	}

	got, ok := names[teamUser.ID]
	if !ok {
		t.Fatalf("generated account has no display name: %+v", names)
	}
	if got.TeamName != "Team Rocket" || got.Institution != "MIT" {
		t.Fatalf("display name = %+v, want Team Rocket / MIT", got)
	}
	if _, ok := names[plain.ID]; ok {
		t.Fatalf("an ordinary user must not get a team label: %+v", names[plain.ID])
	}
}

// An empty page must not open a connection or build an empty IN list.
func TestGetDisplayNamesWithNoUsers(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	names, err := NewContestStore(db).GetDisplayNames(context.Background(), uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("GetDisplayNames: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("want no names, got %+v", names)
	}
}
