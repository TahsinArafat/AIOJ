package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/tahsinarafat/aioj/internal/model"
)

// ProblemI18nStore persists per-problem translations of title/description.
//
// The translation is a *side table* (see migration 000056): a missing row means
// "no translation", and callers fall back to the problem's default title. That
// mirrors dmoj's `Coalesce(F('i18n_translation__name'), F('name'))`.
type ProblemI18nStore struct {
	db *sql.DB
}

func NewProblemI18nStore(db *sql.DB) *ProblemI18nStore {
	return &ProblemI18nStore{db: db}
}

// Get returns the translation for one problem+language.
// Returns (nil, nil) when no translation exists -- absence is not an error.
func (s *ProblemI18nStore) Get(ctx context.Context, problemID, language string) (*model.ProblemI18n, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT problem_id, language, title, description, created_at, updated_at
		   FROM problem_i18n WHERE problem_id = $1 AND language = $2`,
		problemID, language)

	var t model.ProblemI18n
	err := row.Scan(&t.ProblemID, &t.Language, &t.Title, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get problem_i18n: %w", err)
	}
	return &t, nil
}

// ListForProblem returns every translation for a problem, ordered by language so
// responses are deterministic (the admin panel renders them as a list).
func (s *ProblemI18nStore) ListForProblem(ctx context.Context, problemID string) ([]model.ProblemI18n, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT problem_id, language, title, description, created_at, updated_at
		   FROM problem_i18n WHERE problem_id = $1 ORDER BY language`,
		problemID)
	if err != nil {
		return nil, fmt.Errorf("list problem_i18n: %w", err)
	}
	defer rows.Close()

	out := []model.ProblemI18n{}
	for rows.Next() {
		var t model.ProblemI18n
		if err := rows.Scan(&t.ProblemID, &t.Language, &t.Title, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan problem_i18n: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Upsert writes a translation, inserting or replacing the row for that language.
// An empty title/description with no other content still creates the row so a
// setter can blank a translation deliberately; deleting is done via Delete.
func (s *ProblemI18nStore) Upsert(ctx context.Context, t *model.ProblemI18n) error {
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO problem_i18n (problem_id, language, title, description)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (problem_id, language) DO UPDATE
		    SET title = EXCLUDED.title,
		        description = EXCLUDED.description,
		        updated_at = now()
		 RETURNING created_at, updated_at`,
		t.ProblemID, t.Language, t.Title, t.Description,
	).Scan(&t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert problem_i18n: %w", err)
	}
	return nil
}

// Delete removes a translation. Deleting a non-existent row is not an error, so
// the admin UI can treat "remove translation" as idempotent.
func (s *ProblemI18nStore) Delete(ctx context.Context, problemID, language string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM problem_i18n WHERE problem_id = $1 AND language = $2`,
		problemID, language)
	if err != nil {
		return fmt.Errorf("delete problem_i18n: %w", err)
	}
	return nil
}
