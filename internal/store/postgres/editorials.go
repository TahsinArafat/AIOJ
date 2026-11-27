package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
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

func (s *EditorialStore) UpsertOfficial(ctx context.Context, e *model.Editorial) error {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM editorials WHERE problem_id=$1 AND is_official=true ORDER BY created_at LIMIT 1`, e.ProblemID).Scan(&id)
	if err == sql.ErrNoRows {
		return s.Create(ctx, e)
	}
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE editorials SET user_id=$2,title=$3,content=$4,solution_code=$5,solution_language=$6,approach=$7,time_complexity=$8,space_complexity=$9,updated_at=NOW() WHERE id=$1`, id, e.UserID, e.Title, e.Content, e.SolutionCode, e.SolutionLanguage, e.Approach, e.TimeComplexity, e.SpaceComplexity)
	e.ID = id
	return err
}

// UpsertOfficialSolution writes only the reference solution onto the official
// editorial, leaving any prose the setter wrote intact. If no official editorial
// exists yet, one is created with a placeholder title/content so the solution
// has a home; the setter can fill in the prose later from the Editorials tab.
func (s *EditorialStore) UpsertOfficialSolution(ctx context.Context, e *model.Editorial) error {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM editorials WHERE problem_id=$1 AND is_official=true ORDER BY created_at LIMIT 1`, e.ProblemID).Scan(&id)
	if err == sql.ErrNoRows {
		e.IsOfficial = true
		if e.Title == "" {
			e.Title = "Official Editorial"
		}
		return s.Create(ctx, e)
	}
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE editorials SET solution_code=$2, solution_language=$3, updated_at=NOW() WHERE id=$1`, id, e.SolutionCode, e.SolutionLanguage)
	e.ID = id
	return err
}

// Update rewrites the editable fields of one editorial. `problem_id`,
// `contest_id` and `is_official` are deliberately not written: moving an
// editorial to another problem or promoting it to official changes who may read
// it, which is a decision this endpoint is not the place to make.
func (s *EditorialStore) Update(ctx context.Context, e *model.Editorial) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE editorials
		 SET title=$2, content=$3, solution_code=$4, solution_language=$5,
		     approach=$6, time_complexity=$7, space_complexity=$8, updated_at=NOW()
		 WHERE id=$1`,
		e.ID, e.Title, e.Content, e.SolutionCode, e.SolutionLanguage,
		e.Approach, e.TimeComplexity, e.SpaceComplexity)
	if err != nil {
		return err
	}
	// A caller passing an unknown id would otherwise get a silent 200 for a row
	// that does not exist.
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *EditorialStore) Delete(ctx context.Context, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return sql.ErrNoRows
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM editorials WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetForManage returns an editorial regardless of editorialVisible(). The
// visibility rule answers "may this reader see it", and a contest editorial is
// hidden until the round ends — but its author still has to be able to fix a
// typo in it. Authorization happens in the handler, on the row this returns.
func (s *EditorialStore) GetForManage(ctx context.Context, id string) (*model.Editorial, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, nil
	}
	var e model.Editorial
	err := s.db.QueryRowContext(ctx,
		`SELECT e.id, e.problem_id, e.contest_id, e.user_id, e.title, e.content,
		        e.solution_code, e.solution_language, e.approach, e.time_complexity,
		        e.space_complexity, e.is_official, e.created_at, e.updated_at
		 FROM editorials e
		 WHERE e.id = $1`,
		id).Scan(&e.ID, &e.ProblemID, &e.ContestID, &e.UserID, &e.Title, &e.Content,
		&e.SolutionCode, &e.SolutionLanguage, &e.Approach, &e.TimeComplexity,
		&e.SpaceComplexity, &e.IsOfficial, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *EditorialStore) GetByID(ctx context.Context, id string) (*model.Editorial, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, nil
	}
	var e model.Editorial
	err := s.db.QueryRowContext(ctx,
		`SELECT e.id, e.problem_id, p.title, e.contest_id, e.user_id, u.username, e.title, e.content,
		        e.solution_code, e.solution_language, e.approach, e.time_complexity, e.space_complexity,
		        e.is_official, e.upvotes, e.created_at, e.updated_at
		 FROM editorials e
		 JOIN problems p ON e.problem_id = p.id
		 JOIN users u ON e.user_id = u.id
		 WHERE e.id = $1
		   AND editorialVisible(e.contest_id)`,
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
		 WHERE e.problem_id = $1 AND editorialVisible(e.contest_id)
		 ORDER BY e.is_official DESC, e.upvotes DESC`,
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

// GetOfficialSolution returns the official editorial's reference solution for a
// problem, or nil when none exists. Unlike GetByProblem it selects the solution
// fields, because the Reference Solution tab needs the code itself, not just the
// editorial's title.
func (s *EditorialStore) GetOfficialSolution(ctx context.Context, problemID string) (*model.Editorial, error) {
	var e model.Editorial
	err := s.db.QueryRowContext(ctx,
		`SELECT e.id, e.problem_id, e.solution_code, e.solution_language
		 FROM editorials e
		 WHERE e.problem_id = $1 AND e.is_official = true
		 ORDER BY e.created_at LIMIT 1`,
		problemID).Scan(&e.ID, &e.ProblemID, &e.SolutionCode, &e.SolutionLanguage)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *EditorialStore) List(ctx context.Context, offset, limit int) ([]model.Editorial, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM editorials").Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, e.problem_id, p.title, e.user_id, u.username, e.title, e.is_official, e.upvotes, e.created_at
		 FROM editorials e
		 JOIN problems p ON e.problem_id = p.id
		 JOIN users u ON e.user_id = u.id
		 WHERE editorialVisible(e.contest_id)
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
