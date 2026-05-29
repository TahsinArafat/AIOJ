package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type SystemSettingsStore struct {
	db *sql.DB
}

func NewSystemSettingsStore(db *sql.DB) *SystemSettingsStore {
	return &SystemSettingsStore{db: db}
}

func (s *SystemSettingsStore) List(ctx context.Context) ([]model.SystemSetting, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, value, description, updated_at, updated_by FROM system_settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.SystemSetting
	for rows.Next() {
		var ss model.SystemSetting
		if err := rows.Scan(&ss.Key, &ss.Value, &ss.Description, &ss.UpdatedAt, &ss.UpdatedBy); err != nil {
			return nil, err
		}
		items = append(items, ss)
	}
	if items == nil {
		items = []model.SystemSetting{}
	}
	return items, nil
}

func (s *SystemSettingsStore) Get(ctx context.Context, key string) (*model.SystemSetting, error) {
	var ss model.SystemSetting
	err := s.db.QueryRowContext(ctx,
		`SELECT key, value, description, updated_at, updated_by FROM system_settings WHERE key = $1`, key,
	).Scan(&ss.Key, &ss.Value, &ss.Description, &ss.UpdatedAt, &ss.UpdatedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ss, nil
}

func (s *SystemSettingsStore) Set(ctx context.Context, ss *model.SystemSetting) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO system_settings (key, value, description, updated_at, updated_by)
		 VALUES ($1, $2, $3, NOW(), $4)
		 ON CONFLICT (key) DO UPDATE SET value = $2, description = $3, updated_at = NOW(), updated_by = $4`,
		ss.Key, ss.Value, ss.Description, ss.UpdatedBy)
	return err
}

func (s *SystemSettingsStore) Delete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM system_settings WHERE key = $1`, key)
	return err
}
