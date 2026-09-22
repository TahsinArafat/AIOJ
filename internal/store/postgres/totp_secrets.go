package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type TOTPSecretStore struct{ db *sql.DB }

func NewTOTPSecretStore(db *sql.DB) *TOTPSecretStore { return &TOTPSecretStore{db: db} }

func (s *TOTPSecretStore) Upsert(ctx context.Context, userID, secret string) error {
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO totp_secrets (user_id, secret, enabled) VALUES ($1, $2, FALSE)
        ON CONFLICT (user_id) DO UPDATE SET secret = $2, enabled = FALSE, enabled_at = NULL
    `, userID, secret)
	return err
}

func (s *TOTPSecretStore) Enable(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `
        UPDATE totp_secrets SET enabled = TRUE, enabled_at = NOW() WHERE user_id = $1
    `, userID)
	return err
}

func (s *TOTPSecretStore) Disable(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM totp_secrets WHERE user_id = $1`, userID)
	return err
}

func (s *TOTPSecretStore) Get(ctx context.Context, userID string) (*model.TOTPSecret, error) {
	row := s.db.QueryRowContext(ctx, `
        SELECT user_id, secret, enabled, enabled_at, created_at
        FROM totp_secrets WHERE user_id = $1
    `, userID)
	var t model.TOTPSecret
	if err := row.Scan(&t.UserID, &t.Secret, &t.Enabled, &t.EnabledAt, &t.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

type BackupCodeStore struct{ db *sql.DB }

func NewBackupCodeStore(db *sql.DB) *BackupCodeStore { return &BackupCodeStore{db: db} }

func (s *BackupCodeStore) Create(ctx context.Context, id, userID, codeHash string) error {
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO totp_backup_codes (id, user_id, code_hash) VALUES ($1, $2, $3)
    `, id, userID, codeHash)
	return err
}

func (s *BackupCodeStore) ListActive(ctx context.Context, userID string) ([]store.BackupCodeRow, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, code_hash FROM totp_backup_codes
        WHERE user_id = $1 AND used = FALSE
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []store.BackupCodeRow
	for rows.Next() {
		var r store.BackupCodeRow
		if err := rows.Scan(&r.ID, &r.Hash); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *BackupCodeStore) Consume(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE totp_backup_codes SET used = TRUE WHERE id = $1`, id)
	return err
}
