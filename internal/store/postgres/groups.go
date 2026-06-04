package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"math/big"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

type GroupStore struct {
	db *sql.DB
}

func NewGroupStore(db *sql.DB) *GroupStore {
	return &GroupStore{db: db}
}

func generateInviteCode() (string, error) {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	code := make([]byte, 8)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		code[i] = chars[n.Int64()]
	}
	return string(code), nil
}

func (s *GroupStore) Create(ctx context.Context, g *model.Group) error {
	code, err := generateInviteCode()
	if err != nil {
		return err
	}
	if g.JoinPolicy == "" {
		g.JoinPolicy = "auto_approve"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx,
		`INSERT INTO groups (name, description, is_public, max_members, invite_code, join_policy, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at, updated_at`,
		g.Name, g.Description, g.IsPublic, g.MaxMembers, code, g.JoinPolicy, g.CreatedBy,
	).Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return err
	}
	g.InviteCode = code

	_, err = tx.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'owner')`,
		g.ID, g.CreatedBy)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *GroupStore) GetByID(ctx context.Context, id string) (*model.Group, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, nil
	}
	var g model.Group
	err := s.db.QueryRowContext(ctx,
		`SELECT g.id, g.name, g.description, g.is_public, g.max_members,
		        COALESCE(g.invite_code, ''), g.join_policy,
		        g.created_by, u.username,
		        COUNT(gm.user_id) FILTER (WHERE gm.role NOT IN ('invited', 'requested')),
		        g.created_at, g.updated_at
		 FROM groups g
		 JOIN users u ON g.created_by = u.id
		 LEFT JOIN group_members gm ON g.id = gm.group_id
		 WHERE g.id = $1
		 GROUP BY g.id, u.username`,
		id).Scan(&g.ID, &g.Name, &g.Description, &g.IsPublic, &g.MaxMembers,
		&g.InviteCode, &g.JoinPolicy,
		&g.CreatedBy, &g.CreatorName, &g.MemberCount, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *GroupStore) GetByInviteCode(ctx context.Context, code string) (*model.Group, error) {
	var g model.Group
	err := s.db.QueryRowContext(ctx,
		`SELECT g.id, g.name, g.description, g.is_public, g.max_members,
		        COALESCE(g.invite_code, ''), g.join_policy,
		        g.created_by, u.username,
		        COUNT(gm.user_id) FILTER (WHERE gm.role NOT IN ('invited', 'requested')),
		        g.created_at, g.updated_at
		 FROM groups g
		 JOIN users u ON g.created_by = u.id
		 LEFT JOIN group_members gm ON g.id = gm.group_id
		 WHERE g.invite_code = $1
		 GROUP BY g.id, u.username`,
		code).Scan(&g.ID, &g.Name, &g.Description, &g.IsPublic, &g.MaxMembers,
		&g.InviteCode, &g.JoinPolicy,
		&g.CreatedBy, &g.CreatorName, &g.MemberCount, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *GroupStore) List(ctx context.Context, offset, limit int) ([]model.GroupListItem, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM groups WHERE is_public = true").Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name, g.description, g.is_public,
		        COUNT(gm.user_id) FILTER (WHERE gm.role NOT IN ('invited', 'requested')),
		        g.created_at
		 FROM groups g
		 LEFT JOIN group_members gm ON g.id = gm.group_id
		 WHERE g.is_public = true
		 GROUP BY g.id
		 ORDER BY g.created_at DESC
		 OFFSET $1 LIMIT $2`,
		offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.GroupListItem
	for rows.Next() {
		var g model.GroupListItem
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.IsPublic, &g.MemberCount, &g.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, g)
	}
	if items == nil {
		items = []model.GroupListItem{}
	}
	return items, total, nil
}

func (s *GroupStore) ListByUser(ctx context.Context, userID string) ([]model.GroupListItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name, g.description, g.is_public,
		        COUNT(gm2.user_id) FILTER (WHERE gm2.role NOT IN ('invited', 'requested')),
		        g.created_at
		 FROM groups g
		 JOIN group_members gm ON g.id = gm.group_id
		 LEFT JOIN group_members gm2 ON g.id = gm2.group_id
		 WHERE gm.user_id = $1 AND gm.role NOT IN ('invited', 'requested')
		 GROUP BY g.id
		 ORDER BY g.name`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.GroupListItem
	for rows.Next() {
		var g model.GroupListItem
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.IsPublic, &g.MemberCount, &g.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, g)
	}
	if items == nil {
		items = []model.GroupListItem{}
	}
	return items, nil
}

func (s *GroupStore) Update(ctx context.Context, id string, g *model.Group) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE groups SET name = $1, description = $2, is_public = $3, max_members = $4,
		 join_policy = $5, updated_at = NOW() WHERE id = $6`,
		g.Name, g.Description, g.IsPublic, g.MaxMembers, g.JoinPolicy, id)
	return err
}

func (s *GroupStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM groups WHERE id = $1", id)
	return err
}

func (s *GroupStore) AddMember(ctx context.Context, groupID, userID, role string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		groupID, userID, role)
	return err
}

func (s *GroupStore) RemoveMember(ctx context.Context, groupID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`,
		groupID, userID)
	return err
}

func (s *GroupStore) GetMembers(ctx context.Context, groupID string) ([]model.GroupMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT gm.group_id, gm.user_id, u.username, gm.role, gm.joined_at
		 FROM group_members gm
		 JOIN users u ON gm.user_id = u.id
		 WHERE gm.group_id = $1 AND gm.role NOT IN ('invited', 'requested')
		 ORDER BY gm.joined_at`,
		groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []model.GroupMember
	for rows.Next() {
		var m model.GroupMember
		if err := rows.Scan(&m.GroupID, &m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if members == nil {
		members = []model.GroupMember{}
	}
	return members, nil
}

func (s *GroupStore) IsMember(ctx context.Context, groupID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2 AND role NOT IN ('invited', 'requested'))`,
		groupID, userID).Scan(&exists)
	return exists, err
}

func (s *GroupStore) GetMemberCount(ctx context.Context, groupID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM group_members WHERE group_id = $1 AND role NOT IN ('invited', 'requested')`,
		groupID).Scan(&count)
	return count, err
}

func (s *GroupStore) AddContest(ctx context.Context, groupID, contestID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO group_contests (group_id, contest_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		groupID, contestID)
	return err
}

func (s *GroupStore) RemoveContest(ctx context.Context, groupID, contestID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM group_contests WHERE group_id = $1 AND contest_id = $2`,
		groupID, contestID)
	return err
}

func (s *GroupStore) GetContests(ctx context.Context, groupID string) ([]model.Contest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.title, c.type, c.start_time, c.end_time, c.visible, c.description, c.created_at
		 FROM contests c
		 JOIN group_contests gc ON c.id = gc.contest_id
		 WHERE gc.group_id = $1
		 ORDER BY c.start_time DESC`,
		groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contests []model.Contest
	for rows.Next() {
		var c model.Contest
		if err := rows.Scan(&c.ID, &c.Title, &c.Type, &c.StartTime, &c.EndTime, &c.Visible, &c.Description, &c.CreatedAt); err != nil {
			return nil, err
		}
		contests = append(contests, c)
	}
	if contests == nil {
		contests = []model.Contest{}
	}
	return contests, nil
}

func (s *GroupStore) GetMemberRole(ctx context.Context, groupID, userID string) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx,
		"SELECT role FROM group_members WHERE group_id = $1 AND user_id = $2",
		groupID, userID).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return role, nil
}

func (s *GroupStore) UpdateMemberRole(ctx context.Context, groupID, userID, role string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE group_members SET role = $1 WHERE group_id = $2 AND user_id = $3",
		role, groupID, userID)
	return err
}

func (s *GroupStore) GetPendingMembers(ctx context.Context, groupID string) ([]model.GroupMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT gm.group_id, gm.user_id, u.username, gm.role, gm.joined_at
		 FROM group_members gm JOIN users u ON gm.user_id = u.id
		 WHERE gm.group_id = $1 AND gm.role IN ('invited', 'requested')
		 ORDER BY gm.joined_at`,
		groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []model.GroupMember
	for rows.Next() {
		var m model.GroupMember
		if err := rows.Scan(&m.GroupID, &m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if members == nil {
		members = []model.GroupMember{}
	}
	return members, nil
}

func (s *GroupStore) GetUserPendingInvites(ctx context.Context, userID string) ([]model.GroupPendingInvite, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name, gm.role, gm.joined_at
		 FROM group_members gm JOIN groups g ON g.id = gm.group_id
		 WHERE gm.user_id = $1 AND gm.role IN ('invited', 'requested')
		 ORDER BY gm.joined_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invites []model.GroupPendingInvite
	for rows.Next() {
		var inv model.GroupPendingInvite
		if err := rows.Scan(&inv.GroupID, &inv.GroupName, &inv.Role, &inv.JoinedAt); err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	if invites == nil {
		invites = []model.GroupPendingInvite{}
	}
	return invites, nil
}
