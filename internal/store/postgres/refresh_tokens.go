package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RefreshTokenStore struct{ db *sql.DB }

func NewRefreshTokenStore(db *sql.DB) *RefreshTokenStore { return &RefreshTokenStore{db: db} }

func (s *RefreshTokenStore) Create(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1,$2,$3,$4)`,
		uuid.New().String(), userID, tokenHash, expiresAt)
	return err
}

func (s *RefreshTokenStore) Validate(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id FROM refresh_tokens WHERE token_hash=$1 AND expires_at > NOW()`, tokenHash,
	).Scan(&userID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("token not found or expired")
	}
	return userID, err
}
