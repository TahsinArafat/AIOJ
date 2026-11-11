package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrRefreshTokenSpent reports a token that is unknown or past its expiry.
//
// Callers must be able to tell this apart from a failure to reach the store.
// A spent token is a verdict on the session; a store failure is not, and
// answering 401 for the latter makes a database blip indistinguishable from an
// expired session — the client cannot retry, so it signs the user out.
var ErrRefreshTokenSpent = errors.New("refresh token not found or expired")

type RefreshTokenStore struct{ db *sql.DB }

func NewRefreshTokenStore(db *sql.DB) *RefreshTokenStore { return &RefreshTokenStore{db: db} }

func (s *RefreshTokenStore) Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1,$2,$3,$4)`,
		uuid.New().String(), userID, tokenHash, expiresAt)
	return err
}

// Consume deletes the token and returns its owner. Deleting is what makes
// refresh rotate: the old token is spent the moment it is used, so a replayed
// or concurrently replayed token finds no row and fails.
func (s *RefreshTokenStore) Consume(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := s.db.QueryRowContext(ctx,
		`DELETE FROM refresh_tokens WHERE token_hash=$1 AND expires_at > NOW() RETURNING user_id`,
		tokenHash,
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrRefreshTokenSpent
	}
	if err != nil {
		return "", err
	}
	return userID, nil
}

// RevokeAll invalidates every refresh token issued to a user.
func (s *RefreshTokenStore) RevokeAll(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id=$1`, userID)
	return err
}
