package postgres

import (
	"context"
	"testing"
	"time"

	"encoding/json"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * "Clear this" and "don't touch this" are different requests.
 *
 * max_participants and registration_deadline are nullable, and the update body
 * expressed both intents the same way: an omitted key. A nil pointer cannot tell
 * them apart, so the Settings tab's "no cap" (it omitted the field when the box
 * was empty) was read as "leave the old cap alone" — a setter could set a cap and
 * then never remove it. The handler now decodes the key set as well as the struct.
 *
 * This asserts the store end: that a nil really does clear the column, so the
 * handler's presence check has something meaningful to drive. The handler-side
 * decode is covered by TestContestUpdateClearsNullableFields in the handler
 * package.
 */
func TestContestStoreUpdateClearsNullableFields(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	contestStore := NewContestStore(db)

	owner := &model.User{
		ID: uuid.New().String(), Username: "cap_" + uuid.New().String()[:8],
		Email:        "cap_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed", Role: "setter",
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	cap := 50
	deadline := start.Add(-time.Minute)
	c := &model.Contest{
		ID: uuid.New().String(), Title: "Cap " + uuid.New().String()[:8],
		Type: "acm", Format: "acm", StartTime: start, EndTime: start.Add(2 * time.Hour),
		CreatedBy: owner.ID, Status: model.ContestStatusPublished,
		IsRated: true, RatedForMin: 0, RatedForMax: 9999,
		RegistrationRequired: true,
		MaxParticipants:      &cap,
		RegistrationDeadline: &deadline,
	}
	if err := contestStore.Create(ctx, c); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM contest_permissions WHERE contest_id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)
	}()

	// Both were set on create.
	got, err := contestStore.GetByID(ctx, c.ID)
	if err != nil || got == nil {
		t.Fatalf("get after create: %v", err)
	}
	if got.MaxParticipants == nil || *got.MaxParticipants != 50 {
		t.Fatalf("max_participants after create = %v, want 50", got.MaxParticipants)
	}

	// Clearing means nil, and the store must write that through rather than
	// leaving the column as it was.
	got.MaxParticipants = nil
	got.RegistrationDeadline = nil
	if err := contestStore.Update(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	cleared, err := contestStore.GetByID(ctx, c.ID)
	if err != nil || cleared == nil {
		t.Fatalf("get after clear: %v", err)
	}
	if cleared.MaxParticipants != nil {
		t.Fatalf("max_participants = %v after clearing, want NULL — a cap that cannot be removed", *cleared.MaxParticipants)
	}
	if cleared.RegistrationDeadline != nil {
		t.Fatalf("registration_deadline = %v after clearing, want NULL", *cleared.RegistrationDeadline)
	}
}

// The decode rule the handler relies on: presence is what distinguishes the two
// intents, because both spellings produce a nil pointer from the struct alone.
func TestUpdateBodyDistinguishesNullFromAbsent(t *testing.T) {
	type body struct {
		Max *int `json:"max_participants"`
	}
	decode := func(s string) (present bool, val *int) {
		var b body
		if err := json.Unmarshal([]byte(s), &b); err != nil {
			t.Fatalf("unmarshal %q: %v", s, err)
		}
		var keys map[string]json.RawMessage
		_ = json.Unmarshal([]byte(s), &keys)
		_, ok := keys["max_participants"]
		return ok, b.Max
	}

	if present, val := decode(`{}`); present || val != nil {
		t.Fatalf("omitted key: present=%v val=%v, want absent (leave the column alone)", present, val)
	}
	if present, val := decode(`{"max_participants":null}`); !present || val != nil {
		t.Fatalf("explicit null: present=%v val=%v, want present+nil (clear the column)", present, val)
	}
}
