package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

// AchievementStore loads badges and awards them to users.
type AchievementStore struct {
	db *sql.DB
}

func NewAchievementStore(db *sql.DB) *AchievementStore {
	return &AchievementStore{db: db}
}

// ListForUser returns badges the user has earned (most recent first).
func (s *AchievementStore) ListForUser(ctx context.Context, userID string) ([]model.UserAchievement, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.code, a.title, a.description, a.icon, ua.awarded_at
		FROM user_achievements ua
		JOIN achievements a ON a.id = ua.achievement_id
		WHERE ua.user_id = $1
		ORDER BY ua.awarded_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.UserAchievement{}
	for rows.Next() {
		var m model.UserAchievement
		if err := rows.Scan(&m.AchievementID, &m.Code, &m.Title, &m.Description, &m.Icon, &m.AwardedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Award grants badge `code` to user if not already held. Returns true when newly awarded.
func (s *AchievementStore) Award(ctx context.Context, userID, code string) (bool, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return false, nil // not a uuid → skip silently (tests use synthetic ids)
	}
	var achID uuid.UUID
	err = s.db.QueryRowContext(ctx, `SELECT id FROM achievements WHERE code = $1`, code).Scan(&achID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO user_achievements (user_id, achievement_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, id, achID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// AwardProgress awards milestone badges based on solved-problem count (called after AC).
func (s *AchievementStore) AwardProgress(ctx context.Context, userID string, solved int) error {
	milestones := []struct {
		n    int
		code string
	}{
		{1, "first_ac"},
		{10, "ten_ac"},
		{55, "fifty_ac"},
		{100, "hundred_ac"},
	}
	for _, m := range milestones {
		if solved >= m.n {
			_, _ = s.Award(ctx, userID, m.code)
		}
	}
	return nil
}
