package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type RegistrationStore struct {
	db *sql.DB
}

func NewRegistrationStore(db *sql.DB) *RegistrationStore {
	return &RegistrationStore{db: db}
}

func (s *RegistrationStore) Register(ctx context.Context, contestID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO contest_registrations (contest_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		contestID, userID)
	return err
}

func (s *RegistrationStore) Unregister(ctx context.Context, contestID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM contest_registrations WHERE contest_id = $1 AND user_id = $2`,
		contestID, userID)
	return err
}

func (s *RegistrationStore) IsRegistered(ctx context.Context, contestID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM contest_registrations WHERE contest_id = $1 AND user_id = $2)`,
		contestID, userID).Scan(&exists)
	return exists, err
}

func (s *RegistrationStore) GetRegistrations(ctx context.Context, contestID string) ([]model.ContestRegistration, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT r.contest_id, r.user_id, u.username, r.registered_at
		 FROM contest_registrations r
		 JOIN users u ON r.user_id = u.id
		 WHERE r.contest_id = $1 ORDER BY r.registered_at`,
		contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var registrations []model.ContestRegistration
	for rows.Next() {
		var r model.ContestRegistration
		if err := rows.Scan(&r.ContestID, &r.UserID, &r.Username, &r.RegisteredAt); err != nil {
			return nil, err
		}
		registrations = append(registrations, r)
	}
	if registrations == nil {
		registrations = []model.ContestRegistration{}
	}
	return registrations, nil
}

func (s *RegistrationStore) GetRegistrationCount(ctx context.Context, contestID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM contest_registrations WHERE contest_id = $1`,
		contestID).Scan(&count)
	return count, err
}
