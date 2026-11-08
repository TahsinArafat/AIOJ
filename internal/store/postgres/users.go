package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
)

type UserStore struct{ db *sql.DB }

func NewUserStore(db *sql.DB) *UserStore { return &UserStore{db: db} }

func (s *UserStore) Create(ctx context.Context, user *model.User) error {
	// Both rows in one transaction: a user without a `user_profiles` row is
	// invisible to the leaderboard (and to any other INNER JOIN on the profile),
	// because that row used to be created lazily by GetProfile alone. The
	// lazy path is kept as a repair path, but registration should never rely
	// on somebody opening a profile first.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := tx.QueryRowContext(ctx,
		`INSERT INTO users (id, username, email, password_hash, role, is_bot)
         VALUES ($1,$2,$3,$4,$5,$6) RETURNING created_at, updated_at`,
		user.ID, user.Username, user.Email, user.PasswordHash, user.Role, user.IsBot,
	).Scan(&user.CreatedAt, &user.UpdatedAt); err != nil {
		return err
	}

	// Every NOT NULL column on user_profiles has a default, so the defaults are
	// the correct initial state for a freshly registered user.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO user_profiles (user_id) VALUES ($1)`, user.ID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *UserStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	return s.getBy(ctx, "id", id)
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id,username,email,password_hash,role,is_bot,must_change_password,created_at,updated_at FROM users WHERE LOWER(username)=LOWER($1)`,
		username).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.IsBot,
		&u.MustChangePassword, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.getBy(ctx, "email", email)
}

func (s *UserStore) getBy(ctx context.Context, field, value string) (*model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id,username,email,password_hash,role,is_bot,must_change_password,created_at,updated_at FROM users WHERE %s=$1`, field),
		value).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.IsBot,
		&u.MustChangePassword, &u.CreatedAt, &u.UpdatedAt)
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
		WHERE LOWER(u.username) = LOWER($1)
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

// profileColumns is the one column list GetProfile reads, shared by the initial
// read and the re-read after the profile row is lazily created. The two copies
// had already drifted: `max_rating` and `contest_count` were added to the table
// (migration 000003) and to model.UserProfile, but never selected, so the API
// reported 0 for both no matter what the rating service had written.
const profileColumns = `
	SELECT
		up.user_id,
		u.email,
		COALESCE(up.rating, 0),
		COALESCE(up.max_rating, 0),
		COALESCE(up.contest_count, 0),
		-- Live counts, not the stored columns: nothing in the codebase ever
		-- writes user_profiles.problems_solved / .submissions, so the stored
		-- values are 0 for every account no matter how much it has submitted.
		-- idx_submissions_user_status_problem covers both counts.
		COALESCE((SELECT COUNT(DISTINCT s.problem_id) FROM submissions s
		          WHERE s.user_id = u.id AND s.status = 'ac'), 0),
		COALESCE((SELECT COUNT(*) FROM submissions s WHERE s.user_id = u.id), 0),
		COALESCE(up.bio, ''),
		COALESCE(up.avatar_url, ''),
		COALESCE(up.first_name, ''),
		COALESCE(up.last_name, ''),
		COALESCE(up.country, ''),
		COALESCE(up.city, ''),
		COALESCE(up.organization, ''),
		COALESCE(up.github_url, ''),
		COALESCE(up.show_email, false),
		COALESCE(up.show_tags, true),
		u.created_at
	FROM user_profiles up
	JOIN users u ON u.id = up.user_id
	WHERE up.user_id = $1
`

func (s *UserStore) GetProfile(ctx context.Context, userID string) (*model.UserProfile, error) {
	var p model.UserProfile
	scan := func(row *sql.Row) error {
		return row.Scan(
			&p.UserID, &p.Email,
			&p.Rating, &p.MaxRating, &p.ContestCount, &p.ProblemsSolved, &p.Submissions,
			&p.Bio, &p.AvatarURL,
			&p.FirstName, &p.LastName, &p.Country, &p.City, &p.Organization, &p.GithubURL,
			&p.ShowEmail, &p.ShowTags,
			&p.CreatedAt,
		)
	}

	err := scan(s.db.QueryRowContext(ctx, profileColumns, userID))
	if err == sql.ErrNoRows {
		// Repair path: an account whose profile row is missing (registration
		// creates it, but older rows predate that) gets one, then we re-read
		// through the same column list so the shape cannot diverge.
		if _, insertErr := s.db.ExecContext(ctx,
			`INSERT INTO user_profiles (user_id, rating, problems_solved, submissions, bio, max_rating, contest_count)
			 VALUES ($1, 0, 0, 0, '', 0, 0) ON CONFLICT (user_id) DO NOTHING`,
			userID); insertErr != nil {
			return nil, insertErr
		}
		err = scan(s.db.QueryRowContext(ctx, profileColumns, userID))
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

// UpdatePassword also clears the mandatory-change flag: setting a new password
// is exactly what that flag was waiting for.
func (s *UserStore) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE users SET password_hash=$1, must_change_password=FALSE, updated_at=NOW() WHERE id=$2",
		passwordHash, id)
	return err
}

func (s *UserStore) SetMustChangePassword(ctx context.Context, id string, required bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET must_change_password = $1, updated_at = NOW() WHERE id = $2`, required, id)
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

// adminUserColumns is shared by the listing and the detail read so the two can
// never disagree about what a row contains. Live counts are subqueries for the
// same reason as in profileColumns: nothing writes user_profiles.submissions or
// .problems_solved, so the stored columns are permanently zero.
const adminUserColumns = `
	u.id, u.username, u.email, u.role, u.is_bot,
	u.email_verified, u.must_change_password,
	u.banned_at, COALESCE(u.ban_reason, ''),
	COALESCE(up.rating, 0),
	COALESCE((SELECT COUNT(DISTINCT s.problem_id) FROM submissions s
	          WHERE s.user_id = u.id AND s.status = 'ac'), 0),
	COALESCE((SELECT COUNT(*) FROM submissions s WHERE s.user_id = u.id), 0),
	u.created_at, u.last_login_at`

// ListAdminUsers implements the panel's listing. Every filter is applied in
// SQL, not in Go: the total has to describe the same set as the page.
func (s *UserStore) ListAdminUsers(ctx context.Context, f model.UserFilter) ([]model.AdminUserRow, int, error) {
	var where []string
	var args []interface{}

	if term := strings.TrimSpace(f.Search); term != "" {
		// Substring match on username/email, exact match on id. A bare uuid
		// could also match a username, which is fine — it is still the account
		// the admin pasted.
		args = append(args, "%"+term+"%", term)
		where = append(where, fmt.Sprintf("(u.username ILIKE $%d OR u.email ILIKE $%d OR u.id::text = $%d)",
			len(args)-1, len(args)-1, len(args)))
	}
	if f.Role != "" {
		args = append(args, f.Role)
		where = append(where, fmt.Sprintf("u.role = $%d", len(args)))
	}
	switch f.Status {
	case "banned":
		where = append(where, "u.banned_at IS NOT NULL")
	case "active":
		where = append(where, "u.banned_at IS NULL")
	case "unverified":
		where = append(where, "u.email_verified = FALSE")
	case "must_change_password":
		where = append(where, "u.must_change_password = TRUE")
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}

	// The sort column is chosen from a fixed set, never interpolated from raw
	// input — f.Sort is matched by switch below, so no caller string reaches
	// the query text.
	orderCol := "u.created_at"
	switch f.Sort {
	case "username":
		orderCol = "u.username"
	case "rating":
		orderCol = "COALESCE(up.rating, 0)"
	case "submissions":
		// Reuse the select alias: Postgres allows ordering by output column.
		orderCol = "submissions"
	case "last_login":
		// NULLS LAST so "never signed in" sinks to the bottom of a
		// last-login-ascending list instead of filling the first page.
		orderCol = "u.last_login_at"
	case "created":
		orderCol = "u.created_at"
	}
	dir := "DESC"
	if !f.Desc && f.Sort == "username" {
		dir = "ASC"
	}
	orderSQL := fmt.Sprintf(" ORDER BY %s %s, u.id", orderCol, dir)
	if f.Sort == "last_login" {
		orderSQL = fmt.Sprintf(" ORDER BY u.last_login_at %s NULLS LAST, u.id", dir)
	}

	countQuery := "SELECT COUNT(*) FROM users u LEFT JOIN user_profiles up ON up.user_id = u.id" + whereSQL
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 25
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id%s%s
		OFFSET $%d LIMIT $%d`, adminUserColumns, whereSQL, orderSQL, len(args)+1, len(args)+2)

	rows, err := s.db.QueryContext(ctx, query, append(args, offset, limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []model.AdminUserRow{}
	for rows.Next() {
		var u model.AdminUserRow
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.IsBot,
			&u.EmailVerified, &u.MustChangePassword, &u.BannedAt, &u.BanReason,
			&u.Rating, &u.ProblemsSolved, &u.Submissions, &u.CreatedAt,
			&u.LastLoginAt); err != nil {
			return nil, 0, err
		}
		items = append(items, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *UserStore) GetAdminUser(ctx context.Context, id string) (*model.AdminUserDetail, error) {
	var d model.AdminUserDetail
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT %s,
			COALESCE(up.max_rating, 0),
			COALESCE(up.contest_count, 0),
			COALESCE(up.country, ''),
			COALESCE(up.organization, ''),
			COALESCE(up.github_url, ''),
			COALESCE(up.bio, ''),
			COALESCE(t.enabled, FALSE),
			COALESCE((
				SELECT COUNT(*) FROM refresh_tokens rt
				WHERE rt.user_id = u.id AND rt.expires_at > NOW()
			), 0)
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
		LEFT JOIN totp_secrets t ON t.user_id = u.id
		WHERE u.id = $1`, adminUserColumns), id).Scan(
		&d.ID, &d.Username, &d.Email, &d.Role, &d.IsBot,
		&d.EmailVerified, &d.MustChangePassword, &d.BannedAt, &d.BanReason,
		&d.Rating, &d.ProblemsSolved, &d.Submissions, &d.CreatedAt, &d.LastLoginAt,
		&d.MaxRating, &d.ContestCount, &d.Country, &d.Organization, &d.GithubURL,
		&d.Bio, &d.TOTPEnabled, &d.ActiveSessions,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *UserStore) SetUserBan(ctx context.Context, id string, banned bool, reason string) error {
	if banned {
		_, err := s.db.ExecContext(ctx,
			`UPDATE users SET banned_at = NOW(), ban_reason = $2, updated_at = NOW() WHERE id = $1`,
			id, strings.TrimSpace(reason))
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET banned_at = NULL, ban_reason = '', updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *UserStore) IsBanned(ctx context.Context, id string) (bool, error) {
	var banned bool
	err := s.db.QueryRowContext(ctx,
		`SELECT banned_at IS NOT NULL FROM users WHERE id = $1`, id).Scan(&banned)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return banned, nil
}

func (s *UserStore) SetEmailVerified(ctx context.Context, id string, verified bool) error {
	if verified {
		_, err := s.db.ExecContext(ctx,
			`UPDATE users SET email_verified = TRUE,
			        email_verified_at = COALESCE(email_verified_at, NOW()),
			        updated_at = NOW()
			  WHERE id = $1`, id)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET email_verified = FALSE, email_verified_at = NULL, updated_at = NOW() WHERE id = $1`, id)
	return err
}

func (s *UserStore) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin' AND banned_at IS NULL`).Scan(&n)
	return n, err
}

func (s *UserStore) TouchLastLogin(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET last_login_at = NOW() WHERE id = $1`, id)
	return err
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

	// LEFT JOIN, not INNER: a user with no `user_profiles` row (legacy data, or
	// a registration that predates the row being created in Create) must still
	// appear with zeroes rather than vanishing from the leaderboard. The country
	// and organization filters need the join to be a LEFT one too, or a user
	// with no profile silently disappears from a filtered view.
	countQuery := "SELECT COUNT(*) FROM users u LEFT JOIN user_profiles up ON up.user_id = u.id WHERE u.role != 'admin'" + whereClause
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT u.id, u.username, COALESCE(up.rating, 0), COALESCE(up.contest_count, 0),
			COALESCE((
				SELECT rh.rating_change FROM rating_history rh
				WHERE rh.user_id = u.id ORDER BY rh.created_at DESC LIMIT 1
			), 0),
			COALESCE(up.country, '')
		FROM users u
		LEFT JOIN user_profiles up ON up.user_id = u.id
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
		if err := rows.Scan(&e.ID, &e.Username, &e.Rating, &e.ContestsPlayed, &e.RatingChange, &e.Country); err != nil {
			return nil, 0, err
		}
		items = append(items, e)
	}
	if items == nil {
		items = []model.RankingEntry{}
	}
	return items, total, nil
}

func (s *UserStore) MarkEmailVerified(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET email_verified = TRUE, email_verified_at = NOW() WHERE id = $1`, userID)
	return err
}

func (s *UserStore) IsEmailVerified(ctx context.Context, userID string) (bool, error) {
	var v bool
	err := s.db.QueryRowContext(ctx,
		`SELECT email_verified FROM users WHERE id = $1`, userID).Scan(&v)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v, nil
}

func (s *UserStore) UpdateRating(ctx context.Context, userID string, rating, maxRating, contestCount int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE user_profiles SET rating = $1, max_rating = $2, contest_count = $3 WHERE user_id = $4`,
		rating, maxRating, contestCount, userID)
	return err
}
