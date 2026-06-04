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

func (s *UserStore) GetPublicProfile(ctx context.Context, username string) (*model.PublicProfile, error) {
	var p model.PublicProfile
	err := s.db.QueryRowContext(ctx, `
		SELECT
			u.id,
			u.username,
			CASE WHEN up.show_email THEN u.email ELSE '' END AS email,
			COALESCE(up.rating, 0),
			u.created_at,
			COALESCE((
				SELECT rh.rating_change FROM rating_history rh
				WHERE rh.user_id = u.id
				ORDER BY rh.created_at DESC LIMIT 1
			), 0) AS rating_change,
			COALESCE((
				SELECT COUNT(DISTINCT s.contest_id) FROM submissions s
				WHERE s.user_id = u.id AND s.contest_id IS NOT NULL
			), 0) AS contests_played,
			COALESCE((
				SELECT COUNT(DISTINCT s.problem_id) FROM submissions s
				WHERE s.user_id = u.id AND s.status = 'ac'
			), 0) AS problems_solved,
			COALESCE(up.bio, ''),
			COALESCE(up.avatar_url, ''),
			COALESCE(up.first_name, ''),
			COALESCE(up.last_name, ''),
			COALESCE(up.country, ''),
			COALESCE(up.city, ''),
			COALESCE(up.organization, ''),
			COALESCE(up.github_url, ''),
			COALESCE(up.show_email, false),
			COALESCE(up.show_tags, true)
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		WHERE u.username = $1
	`, username).Scan(
		&p.ID, &p.Username, &p.Email,
		&p.Rating, &p.CreatedAt,
		&p.RatingChange, &p.ContestsPlayed, &p.ProblemsSolved,
		&p.Bio, &p.AvatarURL,
		&p.FirstName, &p.LastName, &p.Country, &p.City, &p.Organization, &p.GithubURL,
		&p.ShowEmail, &p.ShowTags,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *UserStore) GetProfile(ctx context.Context, userID string) (*model.UserProfile, error) {
	var p model.UserProfile
	err := s.db.QueryRowContext(ctx, `
		SELECT
			up.user_id,
			COALESCE(up.rating, 0),
			COALESCE(up.problems_solved, 0),
			COALESCE(up.submissions, 0),
			COALESCE(up.bio, ''),
			COALESCE(up.avatar_url, ''),
			COALESCE(up.first_name, ''),
			COALESCE(up.last_name, ''),
			COALESCE(up.country, ''),
			COALESCE(up.city, ''),
			COALESCE(up.organization, ''),
			COALESCE(up.github_url, ''),
			COALESCE(up.show_email, false),
			COALESCE(up.show_tags, true)
		FROM user_profiles up
		WHERE up.user_id = $1
	`, userID).Scan(
		&p.UserID,
		&p.Rating, &p.ProblemsSolved, &p.Submissions,
		&p.Bio, &p.AvatarURL,
		&p.FirstName, &p.LastName, &p.Country, &p.City, &p.Organization, &p.GithubURL,
		&p.ShowEmail, &p.ShowTags,
	)
	if err == sql.ErrNoRows {
		_, insertErr := s.db.ExecContext(ctx,
			`INSERT INTO user_profiles (user_id, rating, problems_solved, submissions, bio, max_rating, contest_count)
			 VALUES ($1, 0, 0, 0, '', 0, 0) ON CONFLICT (user_id) DO NOTHING`,
			userID)
		if insertErr != nil {
			return nil, insertErr
		}
		err = s.db.QueryRowContext(ctx, `
			SELECT
				up.user_id,
				COALESCE(up.rating, 0),
				COALESCE(up.problems_solved, 0),
				COALESCE(up.submissions, 0),
				COALESCE(up.bio, ''),
				COALESCE(up.avatar_url, ''),
				COALESCE(up.first_name, ''),
				COALESCE(up.last_name, ''),
				COALESCE(up.country, ''),
				COALESCE(up.city, ''),
				COALESCE(up.organization, ''),
				COALESCE(up.github_url, ''),
				COALESCE(up.show_email, false),
				COALESCE(up.show_tags, true)
			FROM user_profiles up
			WHERE up.user_id = $1
		`, userID).Scan(
			&p.UserID,
			&p.Rating, &p.ProblemsSolved, &p.Submissions,
			&p.Bio, &p.AvatarURL,
			&p.FirstName, &p.LastName, &p.Country, &p.City, &p.Organization, &p.GithubURL,
			&p.ShowEmail, &p.ShowTags,
		)
		if err != nil {
			return nil, err
		}
		return &p, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *UserStore) UpdateProfile(ctx context.Context, userID string, p *model.UserProfile) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE user_profiles SET
			bio = $2,
			avatar_url = $3,
			first_name = $4,
			last_name = $5,
			country = $6,
			city = $7,
			organization = $8,
			github_url = $9,
			show_email = $10,
			show_tags = $11
		WHERE user_id = $1
	`,
		userID,
		p.Bio, p.AvatarURL,
		p.FirstName, p.LastName, p.Country, p.City, p.Organization, p.GithubURL,
		p.ShowEmail, p.ShowTags,
	)
	return err
}

func (s *UserStore) UpdateRole(ctx context.Context, id, role string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET role=$1, updated_at=NOW() WHERE id=$2", role, id)
	return err
}

func (s *UserStore) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET password_hash=$1, updated_at=NOW() WHERE id=$2", passwordHash, id)
	return err
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

func (s *UserStore) ListUsersByRating(ctx context.Context, offset, limit int, country, organization string) ([]model.RankingEntry, int, error) {
	var whereClause string
	var args []interface{}
	argIdx := 1

	if country != "" {
		whereClause += fmt.Sprintf(" AND up.country = $%d", argIdx)
		args = append(args, country)
		argIdx++
	}
	if organization != "" {
		whereClause += fmt.Sprintf(" AND up.organization = $%d", argIdx)
		args = append(args, organization)
		argIdx++
	}

	countQuery := "SELECT COUNT(*) FROM user_profiles up JOIN users u ON u.id = up.user_id WHERE u.role != 'admin'" + whereClause
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT u.id, u.username, COALESCE(up.rating, 0), COALESCE(up.contest_count, 0),
			COALESCE((
				SELECT rh.rating_change FROM rating_history rh
				WHERE rh.user_id = u.id ORDER BY rh.created_at DESC LIMIT 1
			), 0)
		FROM users u
		JOIN user_profiles up ON up.user_id = u.id
		WHERE u.role != 'admin'%s
		ORDER BY COALESCE(up.rating, 0) DESC, u.username ASC
		OFFSET $%d LIMIT $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, offset, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []model.RankingEntry
	for rows.Next() {
		var e model.RankingEntry
		if err := rows.Scan(&e.ID, &e.Username, &e.Rating, &e.ContestsPlayed, &e.RatingChange); err != nil {
			return nil, 0, err
		}
		items = append(items, e)
	}
	if items == nil {
		items = []model.RankingEntry{}
	}
	return items, total, nil
}

func (s *UserStore) UpdateRating(ctx context.Context, userID string, rating, maxRating, contestCount int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE user_profiles SET rating = $1, max_rating = $2, contest_count = $3 WHERE user_id = $4`,
		rating, maxRating, contestCount, userID)
	return err
}
