package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

type ContestStore struct{ db *sql.DB }

func NewContestStore(db *sql.DB) *ContestStore { return &ContestStore{db: db} }

func (s *ContestStore) Create(ctx context.Context, c *model.Contest) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO contests(id,title,type,start_time,end_time,freeze_time,password,visible,description,created_by)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING created_at`,
		c.ID, c.Title, c.Type, c.StartTime, c.EndTime, c.FreezeTime, c.Password, c.Visible, c.Description, c.CreatedBy,
	).Scan(&c.CreatedAt)
}

func (s *ContestStore) GetByID(ctx context.Context, id string) (*model.Contest, error) {
	var c model.Contest
	err := s.db.QueryRowContext(ctx,
		`SELECT id,title,type,start_time,end_time,freeze_time,visible,description,created_by,created_at
		 FROM contests WHERE id=$1`, id).Scan(
		&c.ID, &c.Title, &c.Type, &c.StartTime, &c.EndTime, &c.FreezeTime,
		&c.Visible, &c.Description, &c.CreatedBy, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *ContestStore) List(ctx context.Context, offset, limit int) ([]model.Contest, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM contests WHERE visible=true").Scan(&total)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,title,type,start_time,end_time,visible,description,created_at
		 FROM contests WHERE visible=true ORDER BY start_time DESC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.Contest
	for rows.Next() {
		var c model.Contest
		rows.Scan(&c.ID, &c.Title, &c.Type, &c.StartTime, &c.EndTime, &c.Visible, &c.Description, &c.CreatedAt)
		items = append(items, c)
	}
	if items == nil {
		items = []model.Contest{}
	}
	return items, total, nil
}

func (s *ContestStore) AddProblem(ctx context.Context, contestID, problemID, index string, score, sortOrder int) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO contest_problems(contest_id,problem_id,index,score,sort_order) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`,
		contestID, problemID, index, score, sortOrder)
	return err
}

func (s *ContestStore) GetProblems(ctx context.Context, contestID string) ([]model.ContestProblem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cp.contest_id,cp.problem_id,cp.index,cp.score,cp.sort_order
		 FROM contest_problems cp WHERE cp.contest_id=$1 ORDER BY cp.sort_order`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.ContestProblem
	for rows.Next() {
		var cp model.ContestProblem
		rows.Scan(&cp.ContestID, &cp.ProblemID, &cp.Index, &cp.Score, &cp.SortOrder)
		items = append(items, cp)
	}
	return items, nil
}

func (s *ContestStore) GetScoreboardRows(ctx context.Context, contestID string, beforeTime *time.Time) ([]ScoreboardRow, error) {
	query := `SELECT s.user_id, s.problem_id, s.status, s.score, s.created_at
			  FROM submissions s WHERE s.contest_id=$1`
	args := []interface{}{contestID}
	if beforeTime != nil {
		query += " AND s.created_at < $2"
		args = append(args, *beforeTime)
	}
	query += " ORDER BY s.created_at ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ScoreboardRow
	for rows.Next() {
		var r ScoreboardRow
		rows.Scan(&r.UserID, &r.ProblemID, &r.Status, &r.Score, &r.CreatedAt)
		result = append(result, r)
	}
	return result, nil
}

func (s *ContestStore) GetParticipants(ctx context.Context, contestID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT user_id FROM submissions WHERE contest_id=$1`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *ContestStore) GetUsername(ctx context.Context, userID string) string {
	var username string
	s.db.QueryRowContext(ctx, "SELECT username FROM users WHERE id=$1", userID).Scan(&username)
	return username
}

type ScoreboardRow struct {
	UserID    string
	ProblemID string
	Status    string
	Score     int
	CreatedAt time.Time
}
