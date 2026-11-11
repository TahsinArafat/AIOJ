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

// Delete is scoped to the owner, for the same reason as API keys: the
// unscoped form let any signed-in account destroy another user's integration.
func (s *WebhookStore) Delete(ctx context.Context, id, userID string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM webhooks WHERE id = $1 AND user_id = $2", id, userID)
	return err
}

// RecordDelivery writes one delivery attempt. The table has existed since
// migration 000015 and was never inserted into, so a webhook could be registered
// and never fire with no trace either way.
func (s *WebhookStore) RecordDelivery(ctx context.Context, d *model.WebhookDelivery) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO webhook_deliveries
		   (webhook_id, event_type, payload, status, response_code, response_body, attempts, delivered_at)
		 VALUES ($1,$2,$3,$4,NULLIF($5,0),$6,$7,$8)
		 RETURNING id, created_at`,
		d.WebhookID, d.EventType, d.Payload, d.Status, d.ResponseCode, d.ResponseBody, d.Attempts, d.DeliveredAt,
	).Scan(&d.ID, &d.CreatedAt)
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
