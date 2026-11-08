package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestUserStore_GetProfileReturnsIdentityAndLiveStats is the regression test for
// the settings page showing "Unrated", an empty email and 0 solved for every
// account:
//
//   - max_rating and contest_count exist on the table (migration 000003) and on
//     model.UserProfile, but the SELECT never listed them, so the API always
//     returned 0 even though the rating service writes them via UpdateRating.
//   - email and created_at live on `users` and were never joined in, yet the
//     page needs both to render the identity header.
//   - problems_solved and submissions are stored columns that nothing in the
//     codebase ever writes, so they were permanently 0. They are now counted
//     from `submissions` instead.
func TestUserStore_GetProfileReturnsIdentityAndLiveStats(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	store := NewUserStore(db)

	suffix := uuid.New().String()[:8]
	user := &model.User{
		ID:           uuid.New().String(),
		Username:     "profilestats_" + suffix,
		Email:        "profilestats_" + suffix + "@example.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	if err := store.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)

	// A rating write, exactly as internal/rating does after a contest.
	if err := store.UpdateRating(ctx, user.ID, 1500, 1610, 7); err != nil {
		t.Fatalf("update rating: %v", err)
	}

	problemID := uuid.New().String()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO problems (id, slug, title, description, created_by) VALUES ($1,$2,$3,$4,$5)`,
		problemID, "profilestats-"+suffix, "Profile stats", "x", user.ID,
	); err != nil {
		t.Fatalf("create problem: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM problems WHERE id = $1", problemID)

	// Two submissions for one problem: one accepted, one wrong answer. The
	// profile counts submissions (2) and distinct solved problems (1).
	for _, status := range []string{"ac", "wa"} {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO submissions (id, problem_id, user_id, language, source_code, status)
			 VALUES ($1,$2,$3,'cpp-gpp-64','int main(){}',$4::verdict)`,
			uuid.New().String(), problemID, user.ID, status,
		); err != nil {
			t.Fatalf("create submission (%s): %v", status, err)
		}
	}
	defer db.ExecContext(ctx, "DELETE FROM submissions WHERE problem_id = $1", problemID)

	profile, err := store.GetProfile(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}

	if profile.Email != user.Email {
		t.Errorf("email = %q, want %q", profile.Email, user.Email)
	}
	if profile.CreatedAt.IsZero() {
		t.Error("created_at is zero: users.created_at is not being selected")
	}
	if profile.Rating != 1500 || profile.MaxRating != 1610 || profile.ContestCount != 7 {
		t.Errorf("rating=%d max=%d contests=%d, want 1500/1610/7",
			profile.Rating, profile.MaxRating, profile.ContestCount)
	}
	if profile.ProblemsSolved != 1 {
		t.Errorf("problems_solved = %d, want 1 (distinct accepted problems)", profile.ProblemsSolved)
	}
	if profile.Submissions != 2 {
		t.Errorf("submissions = %d, want 2", profile.Submissions)
	}
}
