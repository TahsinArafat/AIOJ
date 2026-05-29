package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type PrintStore struct {
	db *sql.DB
}

func NewPrintStore(db *sql.DB) *PrintStore {
	return &PrintStore{db: db}
}

func (s *PrintStore) Create(ctx context.Context, contestID, userID, filename, content string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO print_requests(contest_id, user_id, filename, content)
		 VALUES($1, $2, $3, $4)`,
		contestID, userID, filename, content,
	)
	return err
}

func (s *PrintStore) ListByContest(ctx context.Context, contestID string) ([]model.PrintRequest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT pr.id, pr.contest_id, pr.user_id, u.username, pr.filename, pr.content, pr.status, pr.created_at, pr.updated_at
		 FROM print_requests pr
		 JOIN users u ON pr.user_id = u.id
		 WHERE pr.contest_id = $1
		 ORDER BY pr.created_at DESC`, contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.PrintRequest
	for rows.Next() {
		var r model.PrintRequest
		err := rows.Scan(&r.ID, &r.ContestID, &r.UserID, &r.Username, &r.Filename, &r.Content, &r.Status, &r.CreatedAt, &r.UpdatedAt)
		if err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	if list == nil {
		list = []model.PrintRequest{}
	}
	return list, nil
}

func (s *PrintStore) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE print_requests SET status = $1, updated_at = NOW() WHERE id = $2`, status, id)
	return err
}
