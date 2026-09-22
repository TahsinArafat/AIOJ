package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/tahsinarafat/aioj/internal/model"
)

type OAuthLinkStore struct{ db *sql.DB }

func NewOAuthLinkStore(db *sql.DB) *OAuthLinkStore { return &OAuthLinkStore{db: db} }

func (s *OAuthLinkStore) Create(ctx context.Context, l *model.OAuthLink) error {
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO oauth_links (id, user_id, provider, provider_user_id)
        VALUES ($1, $2, $3, $4)
    `, l.ID, l.UserID, l.Provider, l.ProviderUserID)
	return err
}

func (s *OAuthLinkStore) GetByProviderUser(ctx context.Context, provider, providerUserID string) (*model.OAuthLink, error) {
	row := s.db.QueryRowContext(ctx, `
        SELECT id, user_id, provider, provider_user_id, linked_at
        FROM oauth_links WHERE provider = $1 AND provider_user_id = $2
    `, provider, providerUserID)
	var l model.OAuthLink
	if err := row.Scan(&l.ID, &l.UserID, &l.Provider, &l.ProviderUserID, &l.LinkedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

func (s *OAuthLinkStore) ListByUser(ctx context.Context, userID string) ([]model.OAuthLink, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, user_id, provider, provider_user_id, linked_at
        FROM oauth_links WHERE user_id = $1
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.OAuthLink
	for rows.Next() {
		var l model.OAuthLink
		if err := rows.Scan(&l.ID, &l.UserID, &l.Provider, &l.ProviderUserID, &l.LinkedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

func (s *OAuthLinkStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM oauth_links WHERE id = $1`, id)
	return err
}
