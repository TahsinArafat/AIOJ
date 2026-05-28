package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

func TestGetRecommendationsDB(t *testing.T) {
	// Connect to test database
	dsn := "postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Initialize stores
	userStore := NewUserStore(db)
	probStore := NewProblemStore(db)

	// Create test user
	userID := uuid.New().String()
	user := &model.User{
		ID:           userID,
		Username:     "rec_int_user_" + uuid.New().String()[:8],
		Email:        "rec_int_user_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	if err := userStore.Create(ctx, user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Clean up user and problem permissions at end
	defer func() {
		db.ExecContext(ctx, "DELETE FROM problem_permissions WHERE user_id = $1", userID)
		db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
	}()

	// Create test problems
	p1ID := uuid.New().String()
	p1 := &model.Problem{
		ID:          p1ID,
		Slug:        "rec-int-dp-" + uuid.New().String()[:8],
		Title:       "Easy DP",
		Description: "Practice dynamic programming",
		Difficulty:  "easy",
		Tags:        []string{"dp", "math"},
		Visible:     true,
		CreatedBy:   userID,
	}
	if err := probStore.Create(ctx, p1); err != nil {
		t.Fatalf("failed to create p1: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", p1ID)
	}()

	p2ID := uuid.New().String()
	p2 := &model.Problem{
		ID:          p2ID,
		Slug:        "rec-int-graphs-" + uuid.New().String()[:8],
		Title:       "Easy Graphs",
		Description: "Practice graphs",
		Difficulty:  "easy",
		Tags:        []string{"graphs"},
		Visible:     true,
		CreatedBy:   userID,
	}
	if err := probStore.Create(ctx, p2); err != nil {
		t.Fatalf("failed to create p2: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", p2ID)
	}()

	// Test GetRecommendations
	res, err := probStore.GetRecommendations(ctx, userID, 1200)
	if err != nil {
		t.Fatalf("GetRecommendations returned error: %v", err)
	}

	if res == nil {
		t.Fatal("expected non-nil response")
	}

	// Verify that we got results
	if len(res.Progression) == 0 {
		t.Errorf("expected progression list to have elements, got 0")
	}

	if len(res.WeakTags.Problems) == 0 {
		t.Errorf("expected weak tag problems to have elements, got 0")
	}

	if len(res.Hybrid) == 0 {
		t.Errorf("expected hybrid list to have elements, got 0")
	}
}
