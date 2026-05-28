package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"

	"github.com/lib/pq"
	"github.com/tahsinarafat/aioj/internal/model"
)

type WebhookStore struct {
	db *sql.DB
}

func NewWebhookStore(db *sql.DB) *WebhookStore {
	return &WebhookStore{db: db}
}

func (s *WebhookStore) Create(ctx context.Context, w *model.Webhook) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO webhooks (user_id, url, secret, events, description)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at, updated_at`,
		w.UserID, w.URL, w.Secret, pq.Array(w.Events), w.Description,
	).Scan(&w.ID, &w.CreatedAt, &w.UpdatedAt)
}

func (s *WebhookStore) GetByUser(ctx context.Context, userID string) ([]model.Webhook, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, url, secret, events, is_active, description, created_at, updated_at
		 FROM webhooks WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var events []string
		if err := rows.Scan(&w.ID, &w.UserID, &w.URL, &w.Secret, pq.Array(&events),
			&w.IsActive, &w.Description, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		w.Events = events
		webhooks = append(webhooks, w)
	}
	if webhooks == nil {
		webhooks = []model.Webhook{}
	}
	return webhooks, nil
}

func (s *WebhookStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM webhooks WHERE id = $1", id)
	return err
}

func (s *WebhookStore) GetByEvent(ctx context.Context, eventType string) ([]model.Webhook, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, url, secret, events, is_active, description, created_at, updated_at
		 FROM webhooks WHERE is_active = true AND $1 = ANY(events)`, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []model.Webhook
	for rows.Next() {
		var w model.Webhook
		var events []string
		if err := rows.Scan(&w.ID, &w.UserID, &w.URL, &w.Secret, pq.Array(&events),
			&w.IsActive, &w.Description, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		w.Events = events
		webhooks = append(webhooks, w)
	}
	if webhooks == nil {
		webhooks = []model.Webhook{}
	}
	return webhooks, nil
}

func GenerateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
