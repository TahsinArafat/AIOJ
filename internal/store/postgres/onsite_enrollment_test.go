package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Enrollment alone is not access.
 *
 * This is the store half of the onsite-team fix. AutoRegister used to write only
 * contest_registrations, and the access policy never consults that table: it
 * answers the password question from contest_access_grants. So a team that signed
 * in successfully was enrolled, redirected to its round, and then refused —
 * "forbidden", which the page renders as "Contest not found", while the
 * participants were already in the room.
 *
 * The round here is password-protected on purpose. That is the case the bug hit,
 * and it is the case a registration-only fix would leave broken: the team cannot
 * type a password nobody gave it.
 */
func TestOnsiteAutoRegisterWritesGrant(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	contestStore := NewContestStore(db)
	onsiteStore := NewOnsiteUserStore(db)
	grantStore := NewContestGrantStore(db)

	owner := &model.User{
		ID: uuid.New().String(), Username: "ons_" + uuid.New().String()[:8],
		Email:        "ons_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: "setter",
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	c := &model.Contest{
		ID: uuid.New().String(), Title: "Onsite " + uuid.New().String()[:8],
		Type: "acm", Format: "acm", StartTime: start, EndTime: start.Add(3 * time.Hour),
		CreatedBy: owner.ID, Status: model.ContestStatusPublished,
		PasswordHash: "$2a$12$notarealhashbutnonempty",
		IsRated:      true, RatedForMin: 0, RatedForMax: 9999,
	}
	if err := contestStore.Create(ctx, c); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM contest_access_grants WHERE contest_id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
	}()

	if err := onsiteStore.AutoRegister(ctx, c.ID, owner.ID); err != nil {
		t.Fatalf("AutoRegister: %v", err)
	}

	granted, err := grantStore.IsGranted(ctx, c.ID, owner.ID)
	if err != nil {
		t.Fatalf("IsGranted: %v", err)
	}
	if !granted {
		t.Fatal("AutoRegister did not grant the round; enrollment alone leaves a " +
			"password round closed to the team it was generated for")
	}

	// Registered too, so the round lists them as a participant on the board.
	var registered bool
	if err := db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM contest_registrations WHERE contest_id=$1 AND user_id=$2)`,
		c.ID, owner.ID).Scan(&registered); err != nil {
		t.Fatalf("registration check: %v", err)
	}
	if !registered {
		t.Fatal("AutoRegister must also enroll the team as a participant")
	}

	// Idempotent: every login repeats this, so a second call must not error.
	if err := onsiteStore.AutoRegister(ctx, c.ID, owner.ID); err != nil {
		t.Fatalf("AutoRegister must be idempotent, got: %v", err)
	}
}

/**
 * A group-restricted round must admit its own onsite teams.
 *
 * This is the third half of the same bug, and the one that survives a grant-only
 * fix. The policy checks group restriction BEFORE the password, so a team that is
 * not in the group is refused and never reaches the grant at all. A round is
 * restricted to a group precisely to name who may compete, and a team with
 * generated credentials is on that list by construction.
 */
func TestOnsiteAutoRegisterJoinsRestrictedGroup(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	contestStore := NewContestStore(db)
	onsiteStore := NewOnsiteUserStore(db)
	groupStore := NewGroupStore(db)

	owner := &model.User{
		ID: uuid.New().String(), Username: "onsg_" + uuid.New().String()[:8],
		Email:        "onsg_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: "setter",
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	g := &model.Group{
		Name: "onsite-" + uuid.New().String()[:8], IsPublic: false,
		JoinPolicy: "manual_approve", CreatedBy: owner.ID,
	}
	if err := groupStore.Create(ctx, g); err != nil {
		t.Fatalf("create group: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM groups WHERE id = $1", g.ID)

	// A distinct account for the team: the setter who created the group is its
	// owner and therefore already a member, which would make the precondition
	// below pass for the wrong reason.
	team := &model.User{
		ID: uuid.New().String(), Username: "team_" + uuid.New().String()[:8],
		Email:        "team_" + uuid.New().String()[:8] + "@onsite.aioj",
		PasswordHash: "hashed", Role: "contestant",
	}
	if err := userStore.Create(ctx, team); err != nil {
		t.Fatalf("create team: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", team.ID)

	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	c := &model.Contest{
		ID: uuid.New().String(), Title: "OnsiteGrp " + uuid.New().String()[:8],
		Type: "acm", Format: "acm", StartTime: start, EndTime: start.Add(3 * time.Hour),
		CreatedBy: owner.ID, Status: model.ContestStatusPublished,
		PasswordHash: "$2a$12$notarealhashbutnonempty",
		IsRated:      true, RatedForMin: 0, RatedForMax: 9999,
	}
	if err := contestStore.Create(ctx, c); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM contest_access_grants WHERE contest_id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
	}()

	if err := groupStore.AddContest(ctx, g.ID, c.ID); err != nil {
		t.Fatalf("restrict contest to group: %v", err)
	}

	// Before enrollment the round is closed, which is the state participants hit.
	allowed, err := contestStore.CheckGroupRestriction(ctx, c.ID, team.ID)
	if err != nil {
		t.Fatalf("CheckGroupRestriction: %v", err)
	}
	if allowed {
		t.Fatal("precondition failed: a non-member must be refused by the group rule")
	}

	if err := onsiteStore.AutoRegister(ctx, c.ID, team.ID); err != nil {
		t.Fatalf("AutoRegister: %v", err)
	}

	allowed, err = contestStore.CheckGroupRestriction(ctx, c.ID, team.ID)
	if err != nil {
		t.Fatalf("CheckGroupRestriction after enroll: %v", err)
	}
	if !allowed {
		t.Fatal("AutoRegister left the team outside the round's group; the group " +
			"rule is checked before the password, so the grant cannot rescue it")
	}
}
