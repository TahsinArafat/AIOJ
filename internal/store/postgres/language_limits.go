package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type LanguageLimitStore struct {
	db *sql.DB
}

func NewLanguageLimitStore(db *sql.DB) *LanguageLimitStore {
	return &LanguageLimitStore{db: db}
}

func (s *LanguageLimitStore) Set(ctx context.Context, limit *model.LanguageLimit) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO language_limits (problem_id, language_id, time_limit_ms, memory_limit_kb)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (problem_id, language_id)
		DO UPDATE SET time_limit_ms = $3, memory_limit_kb = $4`,
		limit.ProblemID, limit.LanguageID, limit.TimeLimitMs, limit.MemoryLimitKB)
	return err
}

func (s *LanguageLimitStore) Get(ctx context.Context, problemID, languageID string) (*model.LanguageLimit, error) {
	var l model.LanguageLimit
	var tlm, mlk sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT problem_id, language_id, time_limit_ms, memory_limit_kb
		FROM language_limits WHERE problem_id = $1 AND language_id = $2`,
		problemID, languageID).Scan(&l.ProblemID, &l.LanguageID, &tlm, &mlk)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if tlm.Valid {
		v := int(tlm.Int64)
		l.TimeLimitMs = &v
	}
	if mlk.Valid {
		v := int(mlk.Int64)
		l.MemoryLimitKB = &v
	}
	return &l, nil
}

func (s *LanguageLimitStore) GetByProblem(ctx context.Context, problemID string) ([]*model.LanguageLimit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT problem_id, language_id, time_limit_ms, memory_limit_kb
		FROM language_limits WHERE problem_id = $1`,
		problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var limits []*model.LanguageLimit
	for rows.Next() {
		var l model.LanguageLimit
		var tlm, mlk sql.NullInt64
		if err := rows.Scan(&l.ProblemID, &l.LanguageID, &tlm, &mlk); err != nil {
			return nil, err
		}
		if tlm.Valid {
			v := int(tlm.Int64)
			l.TimeLimitMs = &v
		}
		if mlk.Valid {
			v := int(mlk.Int64)
			l.MemoryLimitKB = &v
		}
		limits = append(limits, &l)
	}
	return limits, rows.Err()
}

func (s *LanguageLimitStore) Delete(ctx context.Context, problemID, languageID string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM language_limits WHERE problem_id = $1 AND language_id = $2`,
		problemID, languageID)
	return err
}
