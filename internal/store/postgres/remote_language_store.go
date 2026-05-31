package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type RemoteLanguageStore struct {
	db *sql.DB
}

func NewRemoteLanguageStore(db *sql.DB) *RemoteLanguageStore {
	return &RemoteLanguageStore{db: db}
}

func (s *RemoteLanguageStore) ListByPlatform(ctx context.Context, platform string) ([]model.RemoteLanguage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, platform, local_id, remote_id, display_name, enabled, sort_order, inline_comment_prefix
		 FROM remote_languages WHERE platform = $1 ORDER BY sort_order`, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.RemoteLanguage
	for rows.Next() {
		var rl model.RemoteLanguage
		if err := rows.Scan(&rl.ID, &rl.Platform, &rl.LocalID, &rl.RemoteID, &rl.DisplayName, &rl.Enabled, &rl.SortOrder, &rl.InlineCommentPrefix); err != nil {
			return nil, err
		}
		items = append(items, rl)
	}
	if items == nil {
		items = []model.RemoteLanguage{}
	}
	return items, nil
}

func (s *RemoteLanguageStore) GetByID(ctx context.Context, id string) (*model.RemoteLanguage, error) {
	var rl model.RemoteLanguage
	err := s.db.QueryRowContext(ctx,
		`SELECT id, platform, local_id, remote_id, display_name, enabled, sort_order, inline_comment_prefix
		 FROM remote_languages WHERE id = $1`, id).Scan(
		&rl.ID, &rl.Platform, &rl.LocalID, &rl.RemoteID, &rl.DisplayName, &rl.Enabled, &rl.SortOrder, &rl.InlineCommentPrefix)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rl, nil
}

func (s *RemoteLanguageStore) Create(ctx context.Context, rl *model.RemoteLanguage) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO remote_languages (platform, local_id, remote_id, display_name, enabled, sort_order, inline_comment_prefix)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		rl.Platform, rl.LocalID, rl.RemoteID, rl.DisplayName, rl.Enabled, rl.SortOrder, rl.InlineCommentPrefix).Scan(&rl.ID)
}

func (s *RemoteLanguageStore) Update(ctx context.Context, id string, rl *model.RemoteLanguage) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE remote_languages SET local_id = $2, remote_id = $3, display_name = $4, enabled = $5, sort_order = $6
		 WHERE id = $1`,
		id, rl.LocalID, rl.RemoteID, rl.DisplayName, rl.Enabled, rl.SortOrder)
	return err
}

func (s *RemoteLanguageStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM remote_languages WHERE id = $1`, id)
	return err
}
