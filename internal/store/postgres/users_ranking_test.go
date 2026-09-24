package postgres

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestUserStore_CreateMakesProfile is the regression test for the permanently
// empty leaderboard. UserStore.Create only inserted into `users`; the matching
// `user_profiles` row was created lazily by GetProfile the first time somebody
// opened that user's profile. Users whose profile was never opened had no
// profile row, and ListUsersByRating's INNER JOIN silently dropped them — so on
// a seeded database /api/rankings returned {"data":[],"total":0} while
// /api/stats still reported every user, and the home page's "Top Rated Users"
// widget was always empty.
func TestUserStore_CreateMakesProfile(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	store := NewUserStore(db)

	user := &model.User{
		ID:           uuid.New().String(),
		Username:     "profile_" + uuid.New().String()[:8],
		Email:        "profile_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	if err := store.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", user.ID)

	// The profile row must exist immediately, without anyone calling GetProfile.
	var rating, solved, submissions, maxRating, contests int
	err := db.QueryRowContext(ctx,
		`SELECT rating, problems_solved, submissions, max_rating, contest_count
		 FROM user_profiles WHERE user_id = $1`, user.ID,
	).Scan(&rating, &solved, &submissions, &maxRating, &contests)
	if err != nil {
		t.Fatalf("user_profiles row missing straight after Create: %v", err)
	}
	if rating != 0 || solved != 0 || submissions != 0 || maxRating != 0 || contests != 0 {
		t.Errorf("new user should start at zeroed stats, got rating=%d solved=%d submissions=%d max=%d contests=%d",
			rating, solved, submissions, maxRating, contests)
	}
}

// TestUserStore_CreateRollsBackOnProfileFailure pins the transaction boundary:
// a user row without its profile row is exactly the broken state this fixes, so
// a failed profile insert must not leave a half-created user behind.
func TestUserStore_CreateRollsBackOnProfileFailure(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	store := NewUserStore(db)

	user := &model.User{
		ID:           uuid.New().String(),
		Username:     "rollback_" + uuid.New().String()[:8],
		Email:        "rollback_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "user",
	}

	// Make the profile insert fail for exactly this one user id, by baking the id
	// into the trigger body. Scoping it that way leaves other tests registering
	// users unaffected and needs no session pinning.
	// (Dropping the FK instead would not work: removing a constraint does not
	// make an otherwise-valid insert fail.)
	const failFn = "test_fail_profile_insert_for_one_user"
	if _, err := db.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION `+failFn+`() RETURNS trigger AS $$
		BEGIN
			IF NEW.user_id = '`+user.ID+`'::uuid THEN
				RAISE EXCEPTION 'simulated user_profiles insert failure';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		DROP TRIGGER IF EXISTS test_fail_profile_insert ON user_profiles;
		CREATE TRIGGER test_fail_profile_insert
			BEFORE INSERT ON user_profiles
			FOR EACH ROW EXECUTE FUNCTION `+failFn+`();`); err != nil {
		t.Skipf("cannot install failure trigger: %v", err)
	}
	defer func() {
		db.ExecContext(ctx, `DROP TRIGGER IF EXISTS test_fail_profile_insert ON user_profiles`)
		db.ExecContext(ctx, `DROP FUNCTION IF EXISTS `+failFn+`()`)
	}()

	// Create must fail, and must not leave the users row behind.
	if err := store.Create(ctx, user); err == nil {
		t.Fatalf("expected Create to fail when the profile insert fails")
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM users WHERE id = $1`, user.ID).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Errorf("Create left a half-created user behind after the profile insert failed: %d row(s) for %s", count, user.ID)
	}
}

// TestUserStore_ListUsersByRatingIncludesUsersWithoutProfile pins the defensive
// half of the fix. Even if a user somehow has no profile row (legacy data, or a
// registration predating this fix), the leaderboard must show them with zeroes
// rather than dropping them — an INNER JOIN silently returned an empty list,
// which is indistinguishable from "nobody has a rating yet".
func TestUserStore_ListUsersByRatingIncludesUsersWithoutProfile(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	store := NewUserStore(db)

	// Legacy-shaped user: row in `users` only, no profile row.
	legacy := &model.User{
		ID:           uuid.New().String(),
		Username:     "legacy_" + uuid.New().String()[:8],
		Email:        "legacy_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "user",
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO users (id, username, email, password_hash, role, is_bot)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		legacy.ID, legacy.Username, legacy.Email, legacy.PasswordHash, legacy.Role, legacy.IsBot,
	); err != nil {
		t.Fatalf("insert legacy user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", legacy.ID)

	// Give them the top rating so they must sort first and cannot be missed.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO user_profiles (user_id, rating) VALUES ($1, 999999)`, legacy.ID); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM user_profiles WHERE user_id = $1", legacy.ID)

	// Now remove the profile row again to recreate the broken state.
	if _, err := db.ExecContext(ctx, `DELETE FROM user_profiles WHERE user_id = $1`, legacy.ID); err != nil {
		t.Fatalf("delete profile: %v", err)
	}

	_, total, err := store.ListUsersByRating(ctx, 0, 100, "", "")
	if err != nil {
		t.Fatalf("list users by rating: %v", err)
	}
	if total == 0 {
		t.Fatal("ListUsersByRating returned total 0; a user with no profile row was dropped")
	}

	// Page through the whole result set looking for the profile-less user.
	found := false
	for offset := 0; offset < total && !found; offset += 100 {
		items, _, err := store.ListUsersByRating(ctx, offset, 100, "", "")
		if err != nil {
			t.Fatalf("list users by rating (offset %d): %v", offset, err)
		}
		for _, e := range items {
			if e.ID == legacy.ID {
				found = true
				if e.Rating != 0 {
					t.Errorf("profile-less user should report rating 0, got %d", e.Rating)
				}
			}
		}
	}
	if !found {
		t.Errorf("user %s has no user_profiles row and was silently dropped from the leaderboard", legacy.Username)
	}
}

// TestUserStore_ListUsersByRatingExcludesAdmins guards the other half of the
// WHERE clause while the join was being widened, so the fix cannot regress into
// showing admin accounts on the public leaderboard.
func TestUserStore_ListUsersByRatingExcludesAdmins(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	store := NewUserStore(db)

	admin := &model.User{
		ID:           uuid.New().String(),
		Username:     "adm_" + uuid.New().String()[:8],
		Email:        "adm_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "hashed",
		Role:         "admin",
	}
	if err := store.Create(ctx, admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", admin.ID)

	_, total, err := store.ListUsersByRating(ctx, 0, 100, "", "")
	if err != nil {
		t.Fatalf("list users by rating: %v", err)
	}
	for offset := 0; offset < total; offset += 100 {
		items, _, err := store.ListUsersByRating(ctx, offset, 100, "", "")
		if err != nil {
			t.Fatalf("list users by rating (offset %d): %v", offset, err)
		}
		for _, e := range items {
			if e.ID == admin.ID {
				t.Fatalf("admin %s appeared on the public leaderboard", admin.Username)
			}
		}
	}
}

var _ = sql.ErrNoRows
