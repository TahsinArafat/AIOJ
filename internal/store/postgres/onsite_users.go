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
			`INSERT INTO onsite_batch_users (id, contest_id, team_name, institution, username, password, password_hash)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			user.ID, user.ContestID, user.TeamName, user.Institution, user.Username, user.Password, user.PasswordHash,
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
		 FROM onsite_batch_users WHERE LOWER(username) = LOWER($1)`,
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
		`SELECT id, contest_id, team_name, institution, username, password, is_used, used_by, created_at
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
		if err := rows.Scan(&u.ID, &u.ContestID, &u.TeamName, &u.Institution, &u.Username, &u.Password, &u.IsUsed, &u.UsedBy, &u.CreatedAt); err != nil {
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

// DeleteByID is scoped to the contest the handler authorized against. The
// unscoped form deleted by raw id, so a manager of contest A could delete
// contest B's onsite users.
func (s *OnsiteUserStoreImpl) DeleteByID(ctx context.Context, contestID, id string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM onsite_batch_users WHERE id = $1 AND contest_id = $2`, id, contestID)
	return err
}

// AutoRegister enrolls a signed-in onsite team in the round it was generated
// for: it registers the team, grants it the round, and joins it to any group the
// round is restricted to.
//
// All three are needed, because the access policy asks the questions in this
// order and stops at the first refusal:
//
//	group restriction -> password grant -> granted
//
// Registration alone was never enough: the policy does not consult
// contest_registrations at all, so enrolling a team in a password-protected
// round changed nothing. The grant is the answer to the password question — the
// setter created these accounts, so they are already vetted the way the password
// is meant to vet people, and making a competitor type a secret nobody gave them
// was the first half of the bug.
//
// The group membership is the second half, and the one that survives every other
// fix: a group-restricted round is refused BEFORE the password is ever checked,
// so a grant cannot rescue a team that is not in the group. A round is restricted
// to a group precisely to name who may compete, and an onsite team with
// generated credentials is on that list by construction.
func (s *OnsiteUserStoreImpl) AutoRegister(ctx context.Context, contestID, userID string) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO contest_registrations (contest_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		contestID, userID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO contest_access_grants (contest_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		contestID, userID); err != nil {
		return err
	}
	// One statement so a round restricted to several groups cannot leave the team
	// in only some of them. A round with no groups inserts nothing, which is the
	// ordinary public case.
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, role)
		 SELECT gc.group_id, $2, 'member' FROM group_contests gc WHERE gc.contest_id = $1
		 ON CONFLICT DO NOTHING`,
		contestID, userID)
	return err
}
