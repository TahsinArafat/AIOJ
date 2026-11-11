package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestRefreshTokenStore_ConsumeDistinguishesMissFromFailure pins the contract
// the refresh handler depends on.
//
// Consume has one error return for two very different situations, and Refresh
// used to answer 401 for both. A database blip therefore looked exactly like an
// expired session: the client cleared the tokens and signed the user out even
// though the token was still valid and could simply have been retried.
func TestRefreshTokenStore_ConsumeDistinguishesMissFromFailure(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	s := NewRefreshTokenStore(db)
	ctx := context.Background()

	userID := newAdminTestUser(t, NewUserStore(db), "user").ID
	hash := uuid.New().String()
	if err := s.Create(ctx, userID, hash, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("seed live token: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM refresh_tokens WHERE token_hash = $1", hash)
	})

	// A live token is consumed exactly once and yields its owner.
	got, err := s.Consume(ctx, hash)
	if err != nil {
		t.Fatalf("consume live token: %v", err)
	}
	if got != userID {
		t.Fatalf("owner = %q, want %q", got, userID)
	}

	// Presenting it again is a miss, and must be recognisable as one.
	if _, err := s.Consume(ctx, hash); !errors.Is(err, ErrRefreshTokenSpent) {
		t.Fatalf("replay error = %v, want ErrRefreshTokenSpent", err)
	}

	// An unknown token carries the same verdict.
	if _, err := s.Consume(ctx, uuid.New().String()); !errors.Is(err, ErrRefreshTokenSpent) {
		t.Fatalf("unknown token error = %v, want ErrRefreshTokenSpent", err)
	}

	// So does an expired one: the row exists but is past its expiry, and the
	// conditional delete leaves it behind.
	expiredHash := uuid.New().String()
	if err := s.Create(ctx, userID, expiredHash, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("seed expired token: %v", err)
	}
	if _, err := s.Consume(ctx, expiredHash); !errors.Is(err, ErrRefreshTokenSpent) {
		t.Fatalf("expired token error = %v, want ErrRefreshTokenSpent", err)
	}
	_, _ = db.ExecContext(ctx, "DELETE FROM refresh_tokens WHERE token_hash = $1", expiredHash)

	// A store that cannot answer is not a verdict on the token. A canceled
	// context fails the query without touching the row, which is the shape a
	// restarted database or a dropped connection produces.
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = s.Consume(canceled, uuid.New().String())
	if err == nil {
		t.Fatal("expected an error from an unanswered query")
	}
	if errors.Is(err, ErrRefreshTokenSpent) {
		t.Fatalf("a store failure must not be reported as a spent token, got %v", err)
	}
}
