package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type SetterStore struct{ db *sql.DB }

func NewSetterStore(db *sql.DB) *SetterStore { return &SetterStore{db: db} }

func (s *SetterStore) CreateApplication(ctx context.Context, userID, reason string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO setter_applications(user_id, status, reason) VALUES($1, 'pending', $2)
		 ON CONFLICT(user_id) DO UPDATE SET status='pending', reason=$2, created_at=NOW()`,
		userID, reason)
	return err
}

func (s *SetterStore) ListApplications(ctx context.Context) ([]model.SetterApplication, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sa.user_id, u.username, sa.status, sa.reason, sa.created_at 
		 FROM setter_applications sa 
		 JOIN users u ON u.id = sa.user_id 
		 ORDER BY sa.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.SetterApplication
	for rows.Next() {
		var a model.SetterApplication
		if err := rows.Scan(&a.UserID, &a.Username, &a.Status, &a.Reason, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	if items == nil {
		items = []model.SetterApplication{}
	}
	return items, nil
}

func (s *SetterStore) UpdateApplicationStatus(ctx context.Context, userID, status string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE setter_applications SET status=$1 WHERE user_id=$2", status, userID)
	return err
}

func (s *SetterStore) GetApplication(ctx context.Context, userID string) (*model.SetterApplication, error) {
	var a model.SetterApplication
	err := s.db.QueryRowContext(ctx,
		`SELECT sa.user_id, u.username, sa.status, sa.reason, sa.created_at 
		 FROM setter_applications sa 
		 JOIN users u ON u.id = sa.user_id 
		 WHERE sa.user_id=$1`,
		userID).Scan(&a.UserID, &a.Username, &a.Status, &a.Reason, &a.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
