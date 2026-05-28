package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

type APIKeyStore struct {
	db *sql.DB
}

func NewAPIKeyStore(db *sql.DB) *APIKeyStore {
	return &APIKeyStore{db: db}
}

func GenerateAPIKey() (key, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	key = "aioj_" + hex.EncodeToString(b)
	h := sha256.Sum256([]byte(key))
	hash = hex.EncodeToString(h[:])
	return key, hash, nil
}

func (s *APIKeyStore) Create(ctx context.Context, k *model.APIKey) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO api_keys (user_id, key_hash, name, description, rate_limit) 
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		k.UserID, k.KeyHash, k.Name, k.Description, k.RateLimit,
	).Scan(&k.ID, &k.CreatedAt)
}

func (s *APIKeyStore) GetByHash(ctx context.Context, keyHash string) (*model.APIKey, error) {
	var k model.APIKey
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, key_hash, name, description, rate_limit, is_active, last_used_at, created_at
		 FROM api_keys WHERE key_hash = $1 AND is_active = true`,
		keyHash).Scan(&k.ID, &k.UserID, &k.KeyHash, &k.Name, &k.Description,
		&k.RateLimit, &k.IsActive, &k.LastUsedAt, &k.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (s *APIKeyStore) GetByUser(ctx context.Context, userID string) ([]model.APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, key_hash, name, description, rate_limit, is_active, last_used_at, created_at
		 FROM api_keys WHERE user_id = $1 ORDER BY created_at`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []model.APIKey
	for rows.Next() {
		var k model.APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.KeyHash, &k.Name, &k.Description,
			&k.RateLimit, &k.IsActive, &k.LastUsedAt, &k.CreatedAt); err != nil {
			return nil, err
		}
		k.KeyPreview = k.KeyHash[:8] + "..."
		keys = append(keys, k)
	}
	if keys == nil {
		keys = []model.APIKey{}
	}
	return keys, nil
}

func (s *APIKeyStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM api_keys WHERE id = $1", id)
	return err
}

func (s *APIKeyStore) UpdateLastUsed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE api_keys SET last_used_at = NOW() WHERE id = $1", id)
	return err
}

func (s *APIKeyStore) IncrementRequestCount(ctx context.Context, keyID string, windowStart time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_rate_limits (api_key_id, window_start, request_count)
		 VALUES ($1, $2, 1) ON CONFLICT (api_key_id, window_start)
		 DO UPDATE SET request_count = request_count + 1`,
		keyID, windowStart)
	return err
}

func (s *APIKeyStore) GetRequestCount(ctx context.Context, keyID string, windowStart time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(request_count), 0) FROM api_rate_limits WHERE api_key_id = $1 AND window_start = $2",
		keyID, windowStart).Scan(&count)
	return count, err
}
