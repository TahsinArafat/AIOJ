package postgres

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// AuditStore records and lists admin audit entries.
type AuditStore struct {
	db *sql.DB
}

func NewAuditStore(db *sql.DB) *AuditStore {
	return &AuditStore{db: db}
}

// Insert writes one entry; never panics on bad actor UUID (best-effort).
func (s *AuditStore) Insert(ctx context.Context, e *model.AuditEntry) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	if len(e.Detail) == 0 {
		e.Detail = json.RawMessage(`{}`)
	}
	var actor any
	if e.ActorID != "" {
		if id, err := uuid.Parse(e.ActorID); err == nil {
			actor = id
		}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (id, actor_id, actor_name, action, target_type, target_id, detail, ip, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, COALESCE($9, NOW()))`,
		e.ID, actor, e.ActorName, e.Action, e.TargetType, e.TargetID, e.Detail, e.IP, e.CreatedAt)
	return err
}

// List returns newest-first page for the admin UI.
func (s *AuditStore) List(ctx context.Context, offset, limit int) ([]model.AuditEntry, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(actor_id::text, ''), actor_name, action, target_type, target_id, detail, ip, created_at
		FROM audit_log ORDER BY created_at DESC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []model.AuditEntry{}
	for rows.Next() {
		var e model.AuditEntry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.ActorName, &e.Action, &e.TargetType, &e.TargetID, &e.Detail, &e.IP, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}
