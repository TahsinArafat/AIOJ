package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// TestGetUsernamesResolvesManyIds covers the batch lookup that replaced the
// per-participant GetUsername call in the scoreboard and the rating
// recalculation.
//
// On a 500-entry standings page the per-row call was 500 round trips for the
// labels alone. The batch form must return the same names in one query, and it
// must report a failure the single-row form could only swallow.
func TestGetUsernamesResolvesManyIds(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	store := NewContestStore(db)
	userStore := NewUserStore(db)

	ids := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		u := &model.User{
			ID:           uuid.New().String(),
			Username:     "un_" + uuid.New().String()[:8],
			Email:        uuid.New().String()[:8] + "@example.com",
			PasswordHash: "hashed",
			Role:         model.RoleUser,
		}
		if err := userStore.Create(ctx, u); err != nil {
			t.Fatalf("create user: %v", err)
		}
		defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", u.ID)
		ids = append(ids, u.ID)
	}

	names, err := store.GetUsernames(ctx, ids)
	if err != nil {
		t.Fatalf("GetUsernames: %v", err)
	}
	if len(names) != len(ids) {
		t.Fatalf("got %d names, want %d: %+v", len(names), len(ids), names)
	}

	// Each id must resolve to its own username, which is what a caller indexes by.
	for _, id := range ids {
		got, ok := names[id]
		if !ok {
			t.Fatalf("id %s missing from result", id)
		}
		var want string
		if err := db.QueryRowContext(ctx, "SELECT username FROM users WHERE id=$1", id).Scan(&want); err != nil {
			t.Fatalf("read back username: %v", err)
		}
		if got != want {
			t.Fatalf("names[%s] = %q, want %q", id, got, want)
		}
	}
}

// An id with no row is absent rather than empty, so a caller can tell "unknown
// user" from "a user whose name is blank" and fall back deliberately.
func TestGetUsernamesOmitsUnknownIds(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()
	store := NewContestStore(db)

	known := &model.User{
		ID:           uuid.New().String(),
		Username:     "un_" + uuid.New().String()[:8],
		Email:        uuid.New().String()[:8] + "@example.com",
		PasswordHash: "hashed",
		Role:         model.RoleUser,
	}
	if err := NewUserStore(db).Create(ctx, known); err != nil {
		t.Fatalf("create user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", known.ID)

	missing := uuid.New().String()
	names, err := store.GetUsernames(ctx, []string{known.ID, missing})
	if err != nil {
		t.Fatalf("GetUsernames: %v", err)
	}
	if names[known.ID] != known.Username {
		t.Fatalf("names[%s] = %q, want %q", known.ID, names[known.ID], known.Username)
	}
	if _, ok := names[missing]; ok {
		t.Fatalf("an id with no row must be absent, got %q", names[missing])
	}
}

// An empty page must not issue a query with an empty ANY() list.
func TestGetUsernamesWithNoIds(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	names, err := NewContestStore(db).GetUsernames(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetUsernames: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("want no names, got %+v", names)
	}
}
