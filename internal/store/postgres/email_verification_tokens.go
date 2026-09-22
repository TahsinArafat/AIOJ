package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

type EmailVerificationTokenStore struct{ db *sql.DB }

func NewEmailVerificationTokenStore(db *sql.DB) *EmailVerificationTokenStore {
	return &EmailVerificationTokenStore{db: db}
}

func (s *EmailVerificationTokenStore) Create(ctx context.Context, id, userID, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at)
         VALUES ($1, $2, $3, $4)`,
		id, userID, tokenHash, expiresAt)
	return err
}

func (s *EmailVerificationTokenStore) GetByHash(ctx context.Context, tokenHash string) (*model.EmailVerificationToken, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, token_hash, expires_at, used, created_at
         FROM email_verification_tokens WHERE token_hash = $1`, tokenHash)
	var t model.EmailVerificationToken
	if err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.Used, &t.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (s *EmailVerificationTokenStore) MarkUsed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE email_verification_tokens SET used = TRUE WHERE id = $1`, id)
	return err
}
