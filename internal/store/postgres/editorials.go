package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type EditorialStore struct {
	db *sql.DB
}

func NewEditorialStore(db *sql.DB) *EditorialStore {
	return &EditorialStore{db: db}
}

func (s *EditorialStore) Create(ctx context.Context, e *model.Editorial) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO editorials (problem_id, contest_id, user_id, title, content, solution_code, 
		                         solution_language, approach, time_complexity, space_complexity, is_official)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id, created_at, updated_at`,
		e.ProblemID, e.ContestID, e.UserID, e.Title, e.Content, e.SolutionCode,
		e.SolutionLanguage, e.Approach, e.TimeComplexity, e.SpaceComplexity, e.IsOfficial,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

func (s *EditorialStore) GetByID(ctx context.Context, id string) (*model.Editorial, error) {
	var e model.Editorial
	err := s.db.QueryRowContext(ctx,
		`SELECT e.id, e.problem_id, p.title, e.contest_id, e.user_id, u.username, e.title, e.content,
		        e.solution_code, e.solution_language, e.approach, e.time_complexity, e.space_complexity,
		        e.is_official, e.upvotes, e.created_at, e.updated_at
		 FROM editorials e
		 JOIN problems p ON e.problem_id = p.id
		 JOIN users u ON e.user_id = u.id
		 WHERE e.id = $1`,
		id).Scan(&e.ID, &e.ProblemID, &e.ProblemTitle, &e.ContestID, &e.UserID, &e.Username,
		&e.Title, &e.Content, &e.SolutionCode, &e.SolutionLanguage, &e.Approach,
		&e.TimeComplexity, &e.SpaceComplexity, &e.IsOfficial, &e.Upvotes, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *EditorialStore) GetByProblem(ctx context.Context, problemID string) ([]model.Editorial, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, e.problem_id, e.user_id, u.username, e.title, e.approach,
		        e.time_complexity, e.is_official, e.upvotes, e.created_at
		 FROM editorials e JOIN users u ON e.user_id = u.id
		 WHERE e.problem_id = $1 ORDER BY e.is_official DESC, e.upvotes DESC`,
		problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var editorials []model.Editorial
	for rows.Next() {
		var e model.Editorial
		if err := rows.Scan(&e.ID, &e.ProblemID, &e.UserID, &e.Username, &e.Title,
			&e.Approach, &e.TimeComplexity, &e.IsOfficial, &e.Upvotes, &e.CreatedAt); err != nil {
			return nil, err
		}
		editorials = append(editorials, e)
	}
	if editorials == nil {
		editorials = []model.Editorial{}
	}
	return editorials, nil
}

func (s *EditorialStore) List(ctx context.Context, offset, limit int) ([]model.Editorial, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM editorials").Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, e.problem_id, p.title, e.user_id, u.username, e.title, e.is_official, e.upvotes, e.created_at
		 FROM editorials e
		 JOIN problems p ON e.problem_id = p.id
		 JOIN users u ON e.user_id = u.id
		 ORDER BY e.created_at DESC OFFSET $1 LIMIT $2`,
		offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.Editorial
	for rows.Next() {
		var e model.Editorial
		if err := rows.Scan(&e.ID, &e.ProblemID, &e.ProblemTitle, &e.UserID, &e.Username,
			&e.Title, &e.IsOfficial, &e.Upvotes, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, e)
	}
	if items == nil {
		items = []model.Editorial{}
	}
	return items, total, nil
}
