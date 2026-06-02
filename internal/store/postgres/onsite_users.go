package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

type OnsiteUserStoreImpl struct {
	db *sql.DB
}

func NewOnsiteUserStore(db *sql.DB) *OnsiteUserStoreImpl {
	return &OnsiteUserStoreImpl{db: db}
}

func (s *OnsiteUserStoreImpl) CreateBatch(ctx context.Context, contestID string, teams []model.BatchUserRequest) ([]model.OnsiteBatchUser, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var created []model.OnsiteBatchUser

	for _, team := range teams {
		username := fmt.Sprintf("team_%s", uuid.New().String()[:8])
		password := uuid.New().String()[:12]

		hash, err := auth.HashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}

		user := model.OnsiteBatchUser{
			ID:           uuid.New().String(),
			ContestID:    contestID,
			TeamName:     team.TeamName,
			Institution:  team.Institution,
			Username:     username,
			Password:     password,
			PasswordHash: hash,
		}

		_, err = tx.ExecContext(ctx,
			`INSERT INTO onsite_batch_users (id, contest_id, team_name, institution, username, password_hash)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			user.ID, user.ContestID, user.TeamName, user.Institution, user.Username, user.PasswordHash,
		)
		if err != nil {
			return nil, fmt.Errorf("insert user: %w", err)
		}

		created = append(created, user)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return created, nil
}

func (s *OnsiteUserStoreImpl) GetByUsername(ctx context.Context, username string) (*model.OnsiteBatchUser, error) {
	var u model.OnsiteBatchUser
	err := s.db.QueryRowContext(ctx,
		`SELECT id, contest_id, team_name, institution, username, password_hash, is_used, used_by, created_at
		 FROM onsite_batch_users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.ContestID, &u.TeamName, &u.Institution, &u.Username, &u.PasswordHash, &u.IsUsed, &u.UsedBy, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get by username: %w", err)
	}
	return &u, nil
}

func (s *OnsiteUserStoreImpl) MarkUsed(ctx context.Context, id string, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE onsite_batch_users SET is_used = true, used_by = $1 WHERE id = $2`,
		userID, id,
	)
	return err
}

func (s *OnsiteUserStoreImpl) ListByContest(ctx context.Context, contestID string) ([]model.OnsiteBatchUser, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, contest_id, team_name, institution, username, is_used, used_by, created_at
		 FROM onsite_batch_users WHERE contest_id = $1 ORDER BY created_at`,
		contestID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.OnsiteBatchUser
	for rows.Next() {
		var u model.OnsiteBatchUser
		if err := rows.Scan(&u.ID, &u.ContestID, &u.TeamName, &u.Institution, &u.Username, &u.IsUsed, &u.UsedBy, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if users == nil {
		users = []model.OnsiteBatchUser{}
	}
	return users, nil
}

func (s *OnsiteUserStoreImpl) DeleteByContest(ctx context.Context, contestID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM onsite_batch_users WHERE contest_id = $1`,
		contestID,
	)
	return err
}

func (s *OnsiteUserStoreImpl) DeleteByID(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM onsite_batch_users WHERE id = $1`, id)
	return err
}

func (s *OnsiteUserStoreImpl) AutoRegister(ctx context.Context, contestID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO contest_registrations (contest_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		contestID, userID)
	return err
}
