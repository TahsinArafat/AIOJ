package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type RatingStore struct {
	db *sql.DB
}

func NewRatingStore(db *sql.DB) *RatingStore {
	return &RatingStore{db: db}
}

func (s *RatingStore) CreateHistory(ctx context.Context, h *model.RatingHistory) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO rating_history (user_id, contest_id, old_rating, new_rating, rank, rating_change)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		h.UserID, h.ContestID, h.OldRating, h.NewRating, h.Rank, h.RatingChange,
	).Scan(&h.ID, &h.CreatedAt)
}

func (s *RatingStore) GetByUser(ctx context.Context, userID string, limit int) ([]model.RatingHistory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, contest_id, old_rating, new_rating, rank, rating_change, created_at
		 FROM rating_history WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`,
		userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []model.RatingHistory
	for rows.Next() {
		var h model.RatingHistory
		if err := rows.Scan(&h.ID, &h.UserID, &h.ContestID, &h.OldRating, &h.NewRating,
			&h.Rank, &h.RatingChange, &h.CreatedAt); err != nil {
			return nil, err
		}
		histories = append(histories, h)
	}
	if histories == nil {
		histories = []model.RatingHistory{}
	}
	return histories, nil
}

func (s *RatingStore) GetByContest(ctx context.Context, contestID string) ([]model.RatingHistory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, contest_id, old_rating, new_rating, rank, rating_change, created_at
		 FROM rating_history WHERE contest_id = $1 ORDER BY rank`,
		contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []model.RatingHistory
	for rows.Next() {
		var h model.RatingHistory
		if err := rows.Scan(&h.ID, &h.UserID, &h.ContestID, &h.OldRating, &h.NewRating,
			&h.Rank, &h.RatingChange, &h.CreatedAt); err != nil {
			return nil, err
		}
		histories = append(histories, h)
	}
	if histories == nil {
		histories = []model.RatingHistory{}
	}
	return histories, nil
}

func (s *RatingStore) GetLatestByUser(ctx context.Context, userID string) (*model.RatingHistory, error) {
	var h model.RatingHistory
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, contest_id, old_rating, new_rating, rank, rating_change, created_at
		 FROM rating_history WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID).Scan(&h.ID, &h.UserID, &h.ContestID, &h.OldRating, &h.NewRating,
		&h.Rank, &h.RatingChange, &h.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}
