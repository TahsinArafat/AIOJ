package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tahsinarafat/aioj/internal/model"
)

type UserStore struct{ db *sql.DB }

func NewUserStore(db *sql.DB) *UserStore { return &UserStore{db: db} }

func (s *UserStore) Create(ctx context.Context, user *model.User) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO users (id, username, email, password_hash, role, is_bot)
         VALUES ($1,$2,$3,$4,$5,$6) RETURNING created_at, updated_at`,
		user.ID, user.Username, user.Email, user.PasswordHash, user.Role, user.IsBot,
	).Scan(&user.CreatedAt, &user.UpdatedAt)
}

func (s *UserStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	return s.getBy(ctx, "id", id)
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	return s.getBy(ctx, "username", username)
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.getBy(ctx, "email", email)
}

func (s *UserStore) getBy(ctx context.Context, field, value string) (*model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id,username,email,password_hash,role,is_bot,created_at,updated_at FROM users WHERE %s=$1`, field),
		value).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.IsBot, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) ListUsers(ctx context.Context, offset, limit int) ([]model.User, int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT id,username,email,role,is_bot,created_at FROM users ORDER BY created_at DESC OFFSET $1 LIMIT $2",
		offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.IsBot, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, u)
	}
	if items == nil {
		items = []model.User{}
	}
	return items, total, nil
}

func (s *UserStore) UpdateRole(ctx context.Context, id, role string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET role=$1, updated_at=NOW() WHERE id=$2", role, id)
	return err
}
