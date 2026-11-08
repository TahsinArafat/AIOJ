package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// newAdminTestUser inserts a user through the store (which also creates the
// profile row) and schedules its removal.
func newAdminTestUser(t *testing.T, s *UserStore, role string) *model.User {
	t.Helper()
	suffix := uuid.New().String()[:8]
	u := &model.User{
		ID:           uuid.New().String(),
		Username:     "adm_" + suffix,
		Email:        "adm_" + suffix + "@example.com",
		PasswordHash: "hashed",
		Role:         role,
	}
	if err := s.Create(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = s.db.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", u.ID)
	})
	return u
}

// TestListAdminUsers_searchMatchesUsernameEmailAndID covers the three ways an
// administrator identifies an account. The id branch matters because pasting an
// id into the search box is the fastest route to a specific complaint.
func TestListAdminUsers_searchMatchesUsernameEmailAndID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	s := NewUserStore(db)
	ctx := context.Background()

	u := newAdminTestUser(t, s, "user")

	for _, term := range []string{u.Username, u.Email, u.ID} {
		rows, _, err := s.ListAdminUsers(ctx, model.UserFilter{Search: term, Limit: 10})
		if err != nil {
			t.Fatalf("search %q: %v", term, err)
		}
		found := false
		for _, r := range rows {
			if r.ID == u.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("search %q did not return the user (got %d rows)", term, len(rows))
		}
	}
}

// TestListAdminUsers_roleAndStatusFilters: the total must describe the same set
// as the page, or the pagination controls lie about how many pages exist.
func TestListAdminUsers_roleAndStatusFilters(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	s := NewUserStore(db)
	ctx := context.Background()

	banned := newAdminTestUser(t, s, "setter")
	if err := s.SetUserBan(ctx, banned.ID, true, "test ban"); err != nil {
		t.Fatalf("ban: %v", err)
	}
	active := newAdminTestUser(t, s, "setter")

	rows, total, err := s.ListAdminUsers(ctx, model.UserFilter{Status: "banned", Search: banned.Username, Limit: 10})
	if err != nil {
		t.Fatalf("list banned: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("banned search: total=%d rows=%d, want 1/1", total, len(rows))
	}
	if rows[0].BanReason != "test ban" {
		t.Errorf("ban reason = %q, want %q", rows[0].BanReason, "test ban")
	}

	rows, _, err = s.ListAdminUsers(ctx, model.UserFilter{Status: "active", Search: active.Username, Limit: 10})
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != active.ID {
		t.Fatalf("active search returned %d rows, want the active user", len(rows))
	}
	if rows[0].BannedAt != nil {
		t.Error("an active account must have a nil banned_at")
	}
}

// TestListAdminUsers_unverifiedAndMustChangeFilters exercises the two support
// queues the panel exposes.
func TestListAdminUsers_unverifiedAndMustChangeFilters(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	s := NewUserStore(db)
	ctx := context.Background()

	u := newAdminTestUser(t, s, "user")
	if err := s.SetMustChangePassword(ctx, u.ID, true); err != nil {
		t.Fatalf("flag: %v", err)
	}

	rows, _, err := s.ListAdminUsers(ctx, model.UserFilter{Status: "must_change_password", Search: u.Username, Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || !rows[0].MustChangePassword {
		t.Fatalf("must_change_password filter returned %d rows", len(rows))
	}

	// A freshly created account is unverified, so the unverified filter must see it.
	rows, _, err = s.ListAdminUsers(ctx, model.UserFilter{Status: "unverified", Search: u.Username, Limit: 10})
	if err != nil {
		t.Fatalf("list unverified: %v", err)
	}
	if len(rows) != 1 || rows[0].EmailVerified {
		t.Fatalf("unverified filter returned %d rows", len(rows))
	}

	if err := s.SetEmailVerified(ctx, u.ID, true); err != nil {
		t.Fatalf("verify: %v", err)
	}
	rows, _, err = s.ListAdminUsers(ctx, model.UserFilter{Status: "unverified", Search: u.Username, Limit: 10})
	if err != nil {
		t.Fatalf("list unverified after verify: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("verified account still matched the unverified filter")
	}
}

// TestListAdminUsers_sortsAndPaginates pins the ordering contract the panel's
// sort dropdown relies on, including "never signed in sorts last".
func TestListAdminUsers_sortsAndPaginates(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	s := NewUserStore(db)
	ctx := context.Background()

	if err := s.TouchLastLogin(ctx, newAdminTestUser(t, s, "user").ID); err != nil {
		t.Fatalf("touch last login: %v", err)
	}

	rows, _, err := s.ListAdminUsers(ctx, model.UserFilter{Sort: "last_login", Desc: false, Limit: 5})
	if err != nil {
		t.Fatalf("sort by last_login: %v", err)
	}
	sawNeverSignedIn := false
	sawSignedIn := false
	for _, r := range rows {
		if r.LastLoginAt == nil {
			sawNeverSignedIn = true
		} else {
			sawSignedIn = true
			if sawNeverSignedIn {
				t.Fatal("a user who never signed in sorted before one who did")
			}
		}
	}
	if !sawSignedIn {
		t.Error("expected at least one signed-in account in the first page")
	}

	// Username ascending is the one order direction the handler derives from
	// Sort rather than from Order.
	rows, _, err = s.ListAdminUsers(ctx, model.UserFilter{Sort: "username", Desc: false, Limit: 200})
	if err != nil {
		t.Fatalf("sort by username: %v", err)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Username > rows[i].Username {
			t.Fatalf("username order broken at %d: %q > %q", i, rows[i-1].Username, rows[i].Username)
		}
	}

	// Pagination must not repeat or drop rows.
	first, total, err := s.ListAdminUsers(ctx, model.UserFilter{Sort: "username", Limit: 2})
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	second, _, err := s.ListAdminUsers(ctx, model.UserFilter{Sort: "username", Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(first) > 0 && len(second) > 0 {
		seen := map[string]bool{}
		for _, r := range first {
			seen[r.ID] = true
		}
		for _, r := range second {
			if seen[r.ID] {
				t.Fatalf("user %s appears on both pages", r.ID)
			}
		}
	}
	if total < len(first)+len(second) {
		t.Fatalf("total %d smaller than the rows returned", total)
	}
}

// TestGetAdminUser_returnsCountsAndSessionFacts: the detail drawer has to answer
// "what has this account done" and "is anyone signed in as it".
func TestGetAdminUser_returnsCountsAndSessionFacts(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	s := NewUserStore(db)
	ctx := context.Background()

	u := newAdminTestUser(t, s, "user")
	if _, err := db.ExecContext(ctx, `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
	                                  VALUES (gen_random_uuid(), $1, $2, NOW() + INTERVAL '1 hour')`,
		u.ID, uuid.New().String()); err != nil {
		t.Fatalf("insert refresh token: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM refresh_tokens WHERE user_id = $1", u.ID)
	})

	d, err := s.GetAdminUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("get admin user: %v", err)
	}
	if d == nil {
		t.Fatal("expected a user")
	}
	if d.Username != u.Username || d.Role != "user" {
		t.Fatalf("unexpected identity: %+v", d)
	}
	if d.ActiveSessions != 1 {
		t.Errorf("active_sessions = %d, want 1", d.ActiveSessions)
	}
	if d.TOTPEnabled {
		t.Error("totp_enabled = true for an account with no enrollment")
	}
}

func TestGetAdminUser_unknownIsNil(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	d, err := NewUserStore(db).GetAdminUser(context.Background(), uuid.New().String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d != nil {
		t.Fatalf("expected nil for an unknown id, got %+v", d)
	}
}

// TestCountAdmins_ignoresBanned keeps the last-admin guard honest: a banned
// administrator cannot sign in, so it must not count as a remaining one.
func TestCountAdmins_ignoresBanned(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	s := NewUserStore(db)
	ctx := context.Background()

	admin := newAdminTestUser(t, s, "admin")

	before, err := s.CountAdmins(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if err := s.SetUserBan(ctx, admin.ID, true, ""); err != nil {
		t.Fatalf("ban: %v", err)
	}
	after, err := s.CountAdmins(ctx)
	if err != nil {
		t.Fatalf("count after ban: %v", err)
	}
	if after != before-1 {
		t.Fatalf("active admins went %d -> %d, want one fewer", before, after)
	}
}

func TestSetUserBan_unbanClearsReason(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	s := NewUserStore(db)
	ctx := context.Background()

	u := newAdminTestUser(t, s, "user")
	if err := s.SetUserBan(ctx, u.ID, true, "because"); err != nil {
		t.Fatalf("ban: %v", err)
	}
	d, err := s.GetAdminUser(ctx, u.ID)
	if err != nil || d == nil {
		t.Fatalf("get: %v", err)
	}
	if d.BanReason != "because" || d.BannedAt == nil {
		t.Fatalf("ban not recorded: %+v", d)
	}

	if err := s.SetUserBan(ctx, u.ID, false, ""); err != nil {
		t.Fatalf("unban: %v", err)
	}
	d, err = s.GetAdminUser(ctx, u.ID)
	if err != nil || d == nil {
		t.Fatalf("get after unban: %v", err)
	}
	if d.BannedAt != nil || d.BanReason != "" {
		t.Fatalf("unban left state behind: %+v", d)
	}
	banned, err := s.IsBanned(ctx, u.ID)
	if err != nil {
		t.Fatalf("is banned: %v", err)
	}
	if banned {
		t.Fatal("IsBanned still true after unban")
	}
}
