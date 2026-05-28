package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

type PasswordResetTokenStore struct{ db *sql.DB }

func NewPasswordResetTokenStore(db *sql.DB) *PasswordResetTokenStore {
	return &PasswordResetTokenStore{db: db}
}

func (s *PasswordResetTokenStore) Create(ctx context.Context, tokenID, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at) VALUES ($1,$2,$3,$4)`,
		tokenID, userID, tokenHash, expiresAt)
	return err
}

func (s *PasswordResetTokenStore) GetByHash(ctx context.Context, tokenHash string) (*model.PasswordResetToken, error) {
	var t model.PasswordResetToken
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, used, created_at FROM password_reset_tokens WHERE token_hash=$1`,
		tokenHash,
	).Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.Used, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *PasswordResetTokenStore) MarkUsed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE password_reset_tokens SET used=TRUE WHERE id=$1", id)
	return err
}
