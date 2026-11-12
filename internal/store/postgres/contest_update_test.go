package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestContestStore_UpdateSettings is the regression test for a settings save
// that could never succeed.
//
// Update derived `visible` from `status` but still passed c.Visible as an
// argument, which left $10 referenced nowhere in the statement. Postgres cannot
// infer the type of a parameter that appears nowhere, so every call failed with
// "could not determine data type of parameter $10" and the handler answered
// 500 "update failed" — for every contest, on every save.
func TestContestStore_UpdateSettings(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	contestStore := NewContestStore(db)

	owner := &model.User{
		ID:           uuid.New().String(),
		Username:     "upd_" + uuid.New().String()[:8],
		Email:        "upd_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed",
		Role:         "setter",
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	c := &model.Contest{
		ID:          uuid.New().String(),
		Title:       "Before",
		Type:        "acm",
		Format:      "acm",
		StartTime:   start,
		EndTime:     start.Add(3 * time.Hour),
		CreatedBy:   owner.ID,
		Status:      model.ContestStatusDraft,
		IsRated:     true,
		RatedForMin: 0,
		RatedForMax: 9999,
	}
	if err := contestStore.Create(ctx, c); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)

	// The shape the setter settings form sends, including a password and the
	// toggles. Visible is set too, and must be ignored.
	c.Title = "Test Round 1"
	c.StartTime = start.Add(2 * time.Hour)
	c.EndTime = c.StartTime.Add(5 * time.Minute)
	// Store-level: the handler hashes before it reaches here, so this is a hash.
	c.PasswordHash = "$2a$12$abcdefghijklmnopqrstuv"
	c.PDFEnabled = true
	c.StatementHidden = false
	c.UpsolvingEnabled = true
	c.VirtualContestEnabled = true
	c.Visible = true

	if err := contestStore.Update(ctx, c); err != nil {
		t.Fatalf("update contest: %v", err)
	}

	got, err := contestStore.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Title != "Test Round 1" {
		t.Fatalf("title = %q, want %q", got.Title, "Test Round 1")
	}
	if got.PasswordHash != c.PasswordHash {
		t.Fatalf("password_hash = %q, want %q", got.PasswordHash, c.PasswordHash)
	}
	if !got.HasPassword {
		t.Fatal("has_password must be derived from a non-empty password_hash")
	}
	if !got.PDFEnabled {
		t.Fatal("pdf_enabled was not persisted")
	}
	if !got.UpsolvingEnabled || !got.VirtualContestEnabled {
		t.Fatalf("flags not persisted: upsolving=%v virtual=%v", got.UpsolvingEnabled, got.VirtualContestEnabled)
	}
	if !got.StartTime.Equal(c.StartTime) || !got.EndTime.Equal(c.EndTime) {
		t.Fatalf("window = [%v, %v], want [%v, %v]", got.StartTime, got.EndTime, c.StartTime, c.EndTime)
	}
	// The reason c.Visible is not an argument: visibility follows status, so a
	// draft stays hidden no matter what the caller sends.
	if got.Visible {
		t.Fatal("a draft must not become visible through Update")
	}
}

// TestContestStore_UpdatePublishesWithStatus pins the other half: an admin
// publishing a round is what makes it visible. Deriving visible from status has
// to work in both directions, or the review workflow would leave published
// contests hidden.
func TestContestStore_UpdatePublishesWithStatus(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	contestStore := NewContestStore(db)

	owner := &model.User{
		ID:           uuid.New().String(),
		Username:     "pub_" + uuid.New().String()[:8],
		Email:        "pub_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed",
		Role:         "setter",
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	c := &model.Contest{
		ID:        uuid.New().String(),
		Title:     "Publish me",
		Type:      "acm",
		Format:    "acm",
		StartTime: start,
		EndTime:   start.Add(time.Hour),
		CreatedBy: owner.ID,
		Status:    model.ContestStatusDraft,
	}
	if err := contestStore.Create(ctx, c); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)

	c.Status = model.ContestStatusPublished
	if err := contestStore.Update(ctx, c); err != nil {
		t.Fatalf("update contest: %v", err)
	}

	got, err := contestStore.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !got.Visible {
		t.Fatal("a published contest must be visible")
	}
}

// TestContestStore_UpdatePersistsRegistrationAndDivision is the regression test
// for settings that could be chosen once and never changed.
//
// registration_required, registration_deadline, max_participants and division were
// all accepted by the create request and then absent from the UPDATE statement.
// The effect was invisible rather than loud: an operator turned registration on,
// the handler assigned the field onto the struct, the save answered 200, and the
// next page load showed the old value. Nothing in the response said the write had
// been dropped.
func TestContestStore_UpdatePersistsRegistrationAndDivision(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	contestStore := NewContestStore(db)

	owner := &model.User{
		ID:           uuid.New().String(),
		Username:     "reg_" + uuid.New().String()[:8],
		Email:        "reg_" + uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed",
		Role:         "setter",
	}
	if err := userStore.Create(ctx, owner); err != nil {
		t.Fatalf("create owner: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", owner.ID)

	start := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	c := &model.Contest{
		ID:          uuid.New().String(),
		Title:       "Registration round",
		Type:        "acm",
		Format:      "acm",
		StartTime:   start,
		EndTime:     start.Add(3 * time.Hour),
		CreatedBy:   owner.ID,
		Status:      model.ContestStatusDraft,
		Division:    model.DivisionNone,
		IsRated:     true,
		RatedForMin: 0,
		RatedForMax: 9999,
	}
	if err := contestStore.Create(ctx, c); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", c.ID)

	deadline := start.Add(-30 * time.Minute).Truncate(time.Second)
	limit := 150
	c.RegistrationRequired = true
	c.RegistrationDeadline = &deadline
	c.MaxParticipants = &limit
	c.Division = model.Division2
	c.Slug = "registered-round"

	if err := contestStore.Update(ctx, c); err != nil {
		t.Fatalf("update contest: %v", err)
	}

	got, err := contestStore.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !got.RegistrationRequired {
		t.Fatal("registration_required was not persisted")
	}
	if got.RegistrationDeadline == nil || !got.RegistrationDeadline.Equal(deadline) {
		t.Fatalf("registration_deadline = %v, want %v", got.RegistrationDeadline, deadline)
	}
	if got.MaxParticipants == nil || *got.MaxParticipants != limit {
		t.Fatalf("max_participants = %v, want %d", got.MaxParticipants, limit)
	}
	if got.Division != model.Division2 {
		t.Fatalf("division = %d, want %d", got.Division, model.Division2)
	}
	if got.Slug != "registered-round" {
		t.Fatalf("slug = %q, want %q", got.Slug, "registered-round")
	}

	// Walking it back has to work too: turning registration off and clearing the
	// cap must reach the column, or a round could only ever gain restrictions.
	c.RegistrationRequired = false
	c.RegistrationDeadline = nil
	c.MaxParticipants = nil
	if err := contestStore.Update(ctx, c); err != nil {
		t.Fatalf("update to clear registration: %v", err)
	}

	got, err = contestStore.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("reload after clearing: %v", err)
	}
	if got.RegistrationRequired {
		t.Fatal("registration_required must be clearable")
	}
	if got.MaxParticipants != nil {
		t.Fatalf("max_participants = %v, want nil", got.MaxParticipants)
	}
}
