package postgres

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

type TeamStore struct {
	db *sql.DB
}

func NewTeamStore(db *sql.DB) *TeamStore {
	return &TeamStore{db: db}
}

func (s *TeamStore) Create(ctx context.Context, t *model.Team) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx,
		`INSERT INTO teams (name, description, created_by, is_public) VALUES ($1, $2, $3, $4) RETURNING id, rating, created_at, updated_at`,
		t.Name, t.Description, t.CreatedBy, t.IsPublic,
	).Scan(&t.ID, &t.Rating, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'owner')",
		t.ID, t.CreatedBy)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *TeamStore) GetByID(ctx context.Context, id string) (*model.Team, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, nil
	}
	var t model.Team
	var avatarURL sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT t.id, t.name, t.description, t.avatar_url, t.rating, t.max_rating, 
		        t.contest_count, COUNT(tm.user_id), t.created_by, u.username, t.is_public, t.created_at, t.updated_at
		 FROM teams t
		 JOIN users u ON t.created_by = u.id
		 LEFT JOIN team_members tm ON t.id = tm.team_id AND tm.role NOT IN ('invited', 'requested')
		 WHERE t.id = $1
		 GROUP BY t.id, u.username`,
		id).Scan(&t.ID, &t.Name, &t.Description, &avatarURL, &t.Rating, &t.MaxRating,
		&t.ContestCount, &t.MemberCount, &t.CreatedBy, &t.CreatorName, &t.IsPublic, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if avatarURL.Valid {
		t.AvatarURL = avatarURL.String
	}
	return &t, nil
}

func (s *TeamStore) List(ctx context.Context, offset, limit int) ([]model.TeamListItem, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM teams WHERE is_public = true").Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, t.rating, COUNT(tm.user_id), t.is_public, t.created_at
		 FROM teams t LEFT JOIN team_members tm ON t.id = tm.team_id AND tm.role NOT IN ('invited', 'requested')
		 WHERE t.is_public = true
		 GROUP BY t.id ORDER BY t.rating DESC OFFSET $1 LIMIT $2`,
		offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.TeamListItem
	for rows.Next() {
		var t model.TeamListItem
		if err := rows.Scan(&t.ID, &t.Name, &t.Rating, &t.MemberCount, &t.IsPublic, &t.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, t)
	}
	if items == nil {
		items = []model.TeamListItem{}
	}
	return items, total, nil
}

func (s *TeamStore) AddMember(ctx context.Context, teamID, userID, role string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		teamID, userID, role)
	return err
}

func (s *TeamStore) RemoveMember(ctx context.Context, teamID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM team_members WHERE team_id = $1 AND user_id = $2",
		teamID, userID)
	return err
}

func (s *TeamStore) GetMembers(ctx context.Context, teamID string) ([]model.TeamMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tm.team_id, tm.user_id, u.username, tm.role, tm.joined_at
		 FROM team_members tm JOIN users u ON tm.user_id = u.id
		 WHERE tm.team_id = $1 AND tm.role NOT IN ('invited', 'requested')
		 ORDER BY tm.joined_at`,
		teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []model.TeamMember
	for rows.Next() {
		var m model.TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if members == nil {
		members = []model.TeamMember{}
	}
	return members, nil
}

func (s *TeamStore) IsMember(ctx context.Context, teamID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM team_members WHERE team_id = $1 AND user_id = $2 AND role NOT IN ('invited', 'requested'))",
		teamID, userID).Scan(&exists)
	return exists, err
}

func (s *TeamStore) GetUserTeams(ctx context.Context, userID string) ([]model.TeamListItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, t.rating, COUNT(tm2.user_id), t.is_public, t.created_at
		 FROM teams t
		 JOIN team_members tm ON t.id = tm.team_id
		 LEFT JOIN team_members tm2 ON t.id = tm2.team_id AND tm2.role NOT IN ('invited', 'requested')
		 WHERE tm.user_id = $1 AND tm.role NOT IN ('invited', 'requested')
		 GROUP BY t.id ORDER BY t.name`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.TeamListItem
	for rows.Next() {
		var t model.TeamListItem
		if err := rows.Scan(&t.ID, &t.Name, &t.Rating, &t.MemberCount, &t.IsPublic, &t.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	if items == nil {
		items = []model.TeamListItem{}
	}
	return items, nil
}

func (s *TeamStore) UpdateRating(ctx context.Context, teamID string, newRating int) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE teams SET rating = $1, max_rating = GREATEST(max_rating, $1), updated_at = NOW() WHERE id = $2",
		newRating, teamID)
	return err
}

func (s *TeamStore) Update(ctx context.Context, id string, t *model.Team) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE teams SET name = $1, description = $2, avatar_url = $3, is_public = $4, updated_at = NOW() WHERE id = $5",
		t.Name, t.Description, t.AvatarURL, t.IsPublic, id)
	return err
}

func (s *TeamStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM teams WHERE id = $1", id)
	return err
}

func (s *TeamStore) GetMemberRole(ctx context.Context, teamID, userID string) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx,
		"SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2",
		teamID, userID).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return role, nil
}

func (s *TeamStore) UpdateMemberRole(ctx context.Context, teamID, userID, role string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE team_members SET role = $1 WHERE team_id = $2 AND user_id = $3",
		role, teamID, userID)
	return err
}

func (s *TeamStore) GetPendingMembers(ctx context.Context, teamID string) ([]model.TeamMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tm.team_id, tm.user_id, u.username, tm.role, tm.joined_at
		 FROM team_members tm JOIN users u ON tm.user_id = u.id
		 WHERE tm.team_id = $1 AND tm.role IN ('invited', 'requested')
		 ORDER BY tm.joined_at`,
		teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []model.TeamMember
	for rows.Next() {
		var m model.TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if members == nil {
		members = []model.TeamMember{}
	}
	return members, nil
}

func (s *TeamStore) GetUserPendingInvites(ctx context.Context, userID string) ([]model.PendingInvite, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, tm.role, tm.joined_at
		 FROM team_members tm JOIN teams t ON t.id = tm.team_id
		 WHERE tm.user_id = $1 AND tm.role IN ('invited', 'requested')
		 ORDER BY tm.joined_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []model.PendingInvite
	for rows.Next() {
		var inv model.PendingInvite
		if err := rows.Scan(&inv.TeamID, &inv.TeamName, &inv.Role, &inv.JoinedAt); err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	if invites == nil {
		invites = []model.PendingInvite{}
	}
	return invites, nil
}
