package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestRatingStore_GetByContestAcceptsDisplayID is the regression test for the
// scoreboard 500: rating_history.contest_id is a UUID, but the URL carries a
// display_id ("12") or a slug. Feeding a display_id straight to GetByContest
// made Postgres raise `invalid input syntax for type uuid` and the public
// scoreboard 500'd on every load. Callers must resolve through ContestStore
// first — this test pins both halves of that contract.
func TestRatingStore_GetByContestAcceptsDisplayID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userStore := NewUserStore(db)
	contestStore := NewContestStore(db)
	ratingStore := NewRatingStore(db)

	userID := uuid.New().String()
	user := &model.User{
		ID:           userID,
		Username:     "rating_test_" + uuid.New().String()[:8],
		Email:        "rating_test_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	if err := userStore.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)

	contestID := uuid.New().String()
	contest := &model.Contest{
		ID:        contestID,
		Title:     "Rating regression " + uuid.New().String()[:6],
		Type:      "acm",
		Format:    "acm",
		StartTime: time.Now().UTC(),
		EndTime:   time.Now().UTC().Add(3 * time.Hour),
		CreatedBy: userID,
	}
	if err := contestStore.Create(ctx, contest); err != nil {
		t.Fatalf("create contest: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM contests WHERE id = $1", contestID)

	// Pin a high display_id so the numeric form of the URL is unambiguous.
	var displayID int
	if err := db.QueryRowContext(ctx, "UPDATE contests SET display_id = 99001 WHERE id = $1 RETURNING display_id", contestID).Scan(&displayID); err != nil {
		t.Fatalf("set display_id: %v", err)
	}

	history := &model.RatingHistory{
		UserID:       userID,
		ContestID:    contestID,
		OldRating:    1200,
		NewRating:    1250,
		Rank:         1,
		RatingChange: 50,
	}
	if err := ratingStore.CreateHistory(ctx, history); err != nil {
		t.Fatalf("create rating history: %v", err)
	}

	t.Run("uuid resolves and returns the row", func(t *testing.T) {
		rows, err := ratingStore.GetByContest(ctx, contestID)
		if err != nil {
			t.Fatalf("GetByContest(uuid): %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("want 1 row for uuid lookup, got %d", len(rows))
		}
		if rows[0].RatingChange != 50 {
			t.Errorf("rating change = %d, want 50", rows[0].RatingChange)
		}
	})

	t.Run("display id is not a uuid and must be rejected", func(t *testing.T) {
		// Documents *why* the handler has to resolve: the raw display_id is
		// unusable as a UUID, which is what caused the 500.
		if _, err := ratingStore.GetByContest(ctx, "99001"); err == nil {
			t.Fatal("expected a uuid type error for a display_id, got nil")
		}
	})

	t.Run("contest store maps display id back to the uuid", func(t *testing.T) {
		resolved, err := contestStore.GetByID(ctx, "99001")
		if err != nil {
			t.Fatalf("GetByID(display_id): %v", err)
		}
		if resolved == nil {
			t.Fatal("GetByID(display_id) returned nil contest")
		}
		if resolved.ID != contestID {
			t.Errorf("resolved id = %s, want %s", resolved.ID, contestID)
		}
	})
}
