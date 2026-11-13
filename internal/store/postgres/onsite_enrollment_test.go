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
