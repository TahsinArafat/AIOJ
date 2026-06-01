package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

type ClarificationStoreImpl struct {
	db *sql.DB
}

func NewClarificationStore(db *sql.DB) *ClarificationStoreImpl {
	return &ClarificationStoreImpl{db: db}
}

func (s *ClarificationStoreImpl) Create(ctx context.Context, c *model.Clarification) error {
	c.ID = uuid.New().String()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO clarifications (id, contest_id, user_id, problem_id, question)
		 VALUES ($1, $2, $3, $4, $5)`,
		c.ID, c.ContestID, c.UserID, c.ProblemID, c.Question,
	)
	return err
}

func (s *ClarificationStoreImpl) GetByID(ctx context.Context, id string) (*model.Clarification, error) {
	var c model.Clarification
	err := s.db.QueryRowContext(ctx,
		`SELECT c.id, c.contest_id, c.user_id, u.username, c.problem_id,
		        c.question, c.answer, c.is_public, c.answered_by, c.created_at, c.updated_at
		 FROM clarifications c
		 JOIN users u ON c.user_id = u.id
		 WHERE c.id = $1`,
		id,
	).Scan(&c.ID, &c.ContestID, &c.UserID, &c.Username, &c.ProblemID,
		&c.Question, &c.Answer, &c.IsPublic, &c.AnsweredBy, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get clarification: %w", err)
	}
	return &c, nil
}

func (s *ClarificationStoreImpl) ListByContest(ctx context.Context, contestID string, userID *string) ([]model.Clarification, error) {
	query := `SELECT c.id, c.contest_id, c.user_id, u.username, c.problem_id,
	                 c.question, c.answer, c.is_public, c.answered_by, c.created_at, c.updated_at
	          FROM clarifications c
	          JOIN users u ON c.user_id = u.id
	          WHERE c.contest_id = $1`
	args := []interface{}{contestID}

	if userID != nil {
		query += " AND (c.is_public = true OR c.user_id = $2)"
		args = append(args, *userID)
	}
	query += " ORDER BY c.created_at DESC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Clarification
	for rows.Next() {
		var c model.Clarification
		if err := rows.Scan(&c.ID, &c.ContestID, &c.UserID, &c.Username, &c.ProblemID,
			&c.Question, &c.Answer, &c.IsPublic, &c.AnsweredBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	if items == nil {
		items = []model.Clarification{}
	}
	return items, nil
}

func (s *ClarificationStoreImpl) Answer(ctx context.Context, id string, answer string, answeredBy string, isPublic bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE clarifications SET answer = $1, answered_by = $2, is_public = $3, updated_at = now()
		 WHERE id = $4`,
		answer, answeredBy, isPublic, id,
	)
	return err
}

type ContestNoticeStoreImpl struct {
	db *sql.DB
}

func NewContestNoticeStore(db *sql.DB) *ContestNoticeStoreImpl {
	return &ContestNoticeStoreImpl{db: db}
}

func (s *ContestNoticeStoreImpl) Create(ctx context.Context, n *model.ContestNotice) error {
	n.ID = uuid.New().String()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO contest_notices (id, contest_id, content, created_by)
		 VALUES ($1, $2, $3, $4)`,
		n.ID, n.ContestID, n.Content, n.CreatedBy,
	)
	return err
}

func (s *ContestNoticeStoreImpl) ListByContest(ctx context.Context, contestID string) ([]model.ContestNotice, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT n.id, n.contest_id, n.content, n.created_by, u.username, n.created_at
		 FROM contest_notices n
		 JOIN users u ON n.created_by = u.id
		 WHERE n.contest_id = $1
		 ORDER BY n.created_at DESC`,
		contestID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.ContestNotice
	for rows.Next() {
		var n model.ContestNotice
		if err := rows.Scan(&n.ID, &n.ContestID, &n.Content, &n.CreatedBy, &n.Username, &n.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	if items == nil {
		items = []model.ContestNotice{}
	}
	return items, nil
}

func (s *ContestNoticeStoreImpl) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM contest_notices WHERE id = $1`, id)
	return err
}
