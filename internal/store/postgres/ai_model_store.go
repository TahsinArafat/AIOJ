package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/tahsinarafat/aioj/internal/model"
)

// AIModelStore is the PostgreSQL implementation of store.AIModelStore.
type AIModelStore struct {
	db *sql.DB
}

// NewAIModelStore creates an AIModelStore backed by db.
func NewAIModelStore(db *sql.DB) *AIModelStore {
	return &AIModelStore{db: db}
}

// List returns all AI models ordered by priority descending.
// api_key is deliberately excluded to avoid leaking secrets in list responses.
func (s *AIModelStore) List(ctx context.Context) ([]model.AIModel, error) {
	const q = `
		SELECT id, name, endpoint, model_name, enabled, priority, description, created_at, updated_at
		FROM ai_models
		ORDER BY priority DESC, created_at ASC`

	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.AIModel
	for rows.Next() {
		var m model.AIModel
		if err := rows.Scan(
			&m.ID, &m.Name, &m.Endpoint, &m.ModelName,
			&m.Enabled, &m.Priority, &m.Description,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetByID returns a single model including its api_key.
func (s *AIModelStore) GetByID(ctx context.Context, id string) (*model.AIModel, error) {
	const q = `
		SELECT id, name, endpoint, api_key, model_name, enabled, priority, description, created_at, updated_at
		FROM ai_models WHERE id = $1`

	var m model.AIModel
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&m.ID, &m.Name, &m.Endpoint, &m.APIKey, &m.ModelName,
		&m.Enabled, &m.Priority, &m.Description,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// GetDefault returns the highest-priority enabled model, or (nil, nil) if none is enabled.
func (s *AIModelStore) GetDefault(ctx context.Context) (*model.AIModel, error) {
	const q = `
		SELECT id, name, endpoint, api_key, model_name, enabled, priority, description, created_at, updated_at
		FROM ai_models
		WHERE enabled = true
		ORDER BY priority DESC, created_at ASC
		LIMIT 1`

	var m model.AIModel
	err := s.db.QueryRowContext(ctx, q).Scan(
		&m.ID, &m.Name, &m.Endpoint, &m.APIKey, &m.ModelName,
		&m.Enabled, &m.Priority, &m.Description,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create inserts a new AIModel and populates its ID, CreatedAt, UpdatedAt.
func (s *AIModelStore) Create(ctx context.Context, am *model.AIModel) error {
	const q = `
		INSERT INTO ai_models (name, endpoint, api_key, model_name, enabled, priority, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`

	return s.db.QueryRowContext(ctx, q,
		am.Name, am.Endpoint, am.APIKey, am.ModelName,
		am.Enabled, am.Priority, am.Description,
	).Scan(&am.ID, &am.CreatedAt, &am.UpdatedAt)
}

// Update replaces all mutable fields on the model identified by id.
// If api_key is empty the existing value is preserved.
func (s *AIModelStore) Update(ctx context.Context, id string, am *model.AIModel) error {
	const q = `
		UPDATE ai_models
		SET name        = $2,
		    endpoint    = $3,
		    api_key     = CASE WHEN $4 = '' THEN api_key ELSE $4 END,
		    model_name  = $5,
		    enabled     = $6,
		    priority    = $7,
		    description = $8,
		    updated_at  = NOW()
		WHERE id = $1
		RETURNING updated_at`

	return s.db.QueryRowContext(ctx, q,
		id,
		am.Name, am.Endpoint, am.APIKey, am.ModelName,
		am.Enabled, am.Priority, am.Description,
	).Scan(&am.UpdatedAt)
}

// Delete removes the model with the given id.
func (s *AIModelStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ai_models WHERE id = $1`, id)
	return err
}

// SetEnabled enables or disables the model with the given id.
func (s *AIModelStore) SetEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE ai_models SET enabled = $2, updated_at = NOW() WHERE id = $1`,
		id, enabled,
	)
	return err
}
