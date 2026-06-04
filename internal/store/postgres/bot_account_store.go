package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/tahsinarafat/aioj/internal/model"
)

type BotAccountStore struct {
	db *sql.DB
}

func NewBotAccountStore(db *sql.DB) *BotAccountStore {
	return &BotAccountStore{db: db}
}

func (s *BotAccountStore) List(ctx context.Context, offset, limit int) ([]model.BotAccount, int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bot_accounts").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, platform, platform_user, status, rate_limit_rps, last_used_at, created_at, proxy_url, proxy_enabled
		 FROM bot_accounts ORDER BY platform, created_at DESC OFFSET $1 LIMIT $2`,
		offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.BotAccount
	for rows.Next() {
		var ba model.BotAccount
		if err := rows.Scan(&ba.ID, &ba.Platform, &ba.PlatformUser, &ba.Status, &ba.RateLimitRPS, &ba.LastUsedAt, &ba.CreatedAt, &ba.ProxyURL, &ba.ProxyEnabled); err != nil {
			return nil, 0, err
		}
		items = append(items, ba)
	}
	if items == nil {
		items = []model.BotAccount{}
	}
	return items, total, nil
}

func (s *BotAccountStore) ListByPlatform(ctx context.Context, platform string) ([]model.BotAccount, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, platform, platform_user, platform_pass, api_key, api_secret, session_data, status, rate_limit_rps, last_used_at, created_at, proxy_url, proxy_enabled
		 FROM bot_accounts WHERE platform = $1 AND status = 'active' ORDER BY last_used_at ASC NULLS FIRST`, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.BotAccount
	for rows.Next() {
		var ba model.BotAccount
		var sd []byte
		if err := rows.Scan(&ba.ID, &ba.Platform, &ba.PlatformUser, &ba.PlatformPass, &ba.APIKey, &ba.APISecret, &sd, &ba.Status, &ba.RateLimitRPS, &ba.LastUsedAt, &ba.CreatedAt, &ba.ProxyURL, &ba.ProxyEnabled); err != nil {
			return nil, err
		}
		if len(sd) > 0 {
			json.Unmarshal(sd, &ba.SessionData)
		}
		items = append(items, ba)
	}
	if items == nil {
		items = []model.BotAccount{}
	}
	return items, nil
}

func (s *BotAccountStore) GetByID(ctx context.Context, id string) (*model.BotAccount, error) {
	var ba model.BotAccount
	var sd []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT id, platform, platform_user, platform_pass, api_key, api_secret, session_data, status, rate_limit_rps, last_used_at, created_at, proxy_url, proxy_enabled
		 FROM bot_accounts WHERE id = $1`, id,
	).Scan(&ba.ID, &ba.Platform, &ba.PlatformUser, &ba.PlatformPass, &ba.APIKey, &ba.APISecret, &sd, &ba.Status, &ba.RateLimitRPS, &ba.LastUsedAt, &ba.CreatedAt, &ba.ProxyURL, &ba.ProxyEnabled)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(sd) > 0 {
		json.Unmarshal(sd, &ba.SessionData)
	}
	return &ba, nil
}

func (s *BotAccountStore) Create(ctx context.Context, ba *model.BotAccount) error {
	sd, _ := json.Marshal(ba.SessionData)
	return s.db.QueryRowContext(ctx,
		`INSERT INTO bot_accounts (user_id, platform, platform_user, platform_pass, api_key, api_secret, session_data, status, rate_limit_rps, proxy_url, proxy_enabled)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, created_at`,
		ba.UserID, ba.Platform, ba.PlatformUser, ba.PlatformPass, ba.APIKey, ba.APISecret, sd, ba.Status, ba.RateLimitRPS, ba.ProxyURL, ba.ProxyEnabled,
	).Scan(&ba.ID, &ba.CreatedAt)
}

func (s *BotAccountStore) Update(ctx context.Context, id string, ba *model.BotAccount) error {
	sd, _ := json.Marshal(ba.SessionData)
	_, err := s.db.ExecContext(ctx,
		`UPDATE bot_accounts SET platform=$1, platform_user=$2, platform_pass=$3, api_key=$4, api_secret=$5, session_data=$6, status=$7, rate_limit_rps=$8, proxy_url=$9, proxy_enabled=$10 WHERE id=$11`,
		ba.Platform, ba.PlatformUser, ba.PlatformPass, ba.APIKey, ba.APISecret, sd, ba.Status, ba.RateLimitRPS, ba.ProxyURL, ba.ProxyEnabled, id)
	return err
}

func (s *BotAccountStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM bot_accounts WHERE id=$1`, id)
	return err
}

func (s *BotAccountStore) IncrementFailures(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE bot_accounts SET consecutive_failures = consecutive_failures + 1, last_error_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *BotAccountStore) ResetFailures(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE bot_accounts SET consecutive_failures = 0, last_error_at = NULL WHERE id = $1`, id)
	return err
}

func (s *BotAccountStore) MarkUsed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE bot_accounts SET last_used_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *BotAccountStore) UpdateLastPoll(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE bot_accounts SET last_poll_at = NOW() WHERE id = $1`, id)
	return err
}
