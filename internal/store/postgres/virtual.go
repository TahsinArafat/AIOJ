package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type VirtualStore struct {
	db *sql.DB
}

func NewVirtualStore(db *sql.DB) *VirtualStore {
	return &VirtualStore{db: db}
}

func (s *VirtualStore) Create(ctx context.Context, v *model.VirtualContest) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO virtual_contests (original_contest_id, user_id, started_at, duration_minutes, status)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		v.OriginalContestID, v.UserID, v.StartedAt, v.DurationMinutes, v.Status,
	).Scan(&v.ID, &v.CreatedAt)
}

func (s *VirtualStore) GetByID(ctx context.Context, id string) (*model.VirtualContest, error) {
	var v model.VirtualContest
	err := s.db.QueryRowContext(ctx,
		`SELECT id, original_contest_id, user_id, started_at, ended_at, duration_minutes, status, created_at
		 FROM virtual_contests WHERE id = $1`,
		id).Scan(&v.ID, &v.OriginalContestID, &v.UserID, &v.StartedAt, &v.EndedAt,
		&v.DurationMinutes, &v.Status, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *VirtualStore) GetActiveByUser(ctx context.Context, userID string) (*model.VirtualContest, error) {
	var v model.VirtualContest
	err := s.db.QueryRowContext(ctx,
		`SELECT id, original_contest_id, user_id, started_at, ended_at, duration_minutes, status, created_at
		 FROM virtual_contests WHERE user_id = $1 AND status = 'active'`,
		userID).Scan(&v.ID, &v.OriginalContestID, &v.UserID, &v.StartedAt, &v.EndedAt,
		&v.DurationMinutes, &v.Status, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *VirtualStore) Complete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE virtual_contests SET status = 'completed', ended_at = NOW() WHERE id = $1", id)
	return err
}
