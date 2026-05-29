package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type ClassStore struct {
	db *sql.DB
}

func NewClassStore(db *sql.DB) *ClassStore {
	return &ClassStore{db: db}
}

func (s *ClassStore) Create(ctx context.Context, c *model.Class) error {
	err := s.db.QueryRowContext(ctx,
		`INSERT INTO classes (organization_id, name, description, invite_code, created_by)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at, updated_at`,
		c.OrganizationID, c.Name, c.Description, c.InviteCode, c.CreatedBy,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	return err
}

func (s *ClassStore) GetByID(ctx context.Context, id string) (*model.Class, error) {
	var c model.Class
	err := s.db.QueryRowContext(ctx,
		`SELECT c.id, c.organization_id, o.name, c.name, c.description, c.invite_code,
		        c.created_by, u.username, COUNT(cm.user_id), c.created_at, c.updated_at
		 FROM classes c
		 JOIN organizations o ON c.organization_id = o.id
		 JOIN users u ON c.created_by = u.id
		 LEFT JOIN class_members cm ON c.id = cm.class_id
		 WHERE c.id = $1
		 GROUP BY c.id, o.name, u.username`,
		id).Scan(&c.ID, &c.OrganizationID, &c.OrgName, &c.Name, &c.Description,
		&c.InviteCode, &c.CreatedBy, &c.CreatorName, &c.StudentCount,
		&c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *ClassStore) GetByInviteCode(ctx context.Context, code string) (*model.Class, error) {
	var c model.Class
	err := s.db.QueryRowContext(ctx,
		`SELECT c.id, c.organization_id, o.name, c.name, c.description, c.invite_code,
		        c.created_by, u.username, COUNT(cm.user_id), c.created_at, c.updated_at
		 FROM classes c
		 JOIN organizations o ON c.organization_id = o.id
		 JOIN users u ON c.created_by = u.id
		 LEFT JOIN class_members cm ON c.id = cm.class_id
		 WHERE c.invite_code = $1
		 GROUP BY c.id, o.name, u.username`,
		code).Scan(&c.ID, &c.OrganizationID, &c.OrgName, &c.Name, &c.Description,
		&c.InviteCode, &c.CreatedBy, &c.CreatorName, &c.StudentCount,
		&c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *ClassStore) List(ctx context.Context, orgID string, offset, limit int) ([]model.ClassListItem, int, error) {
	var total int
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM classes WHERE organization_id = $1", orgID).Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.organization_id, c.name, c.description, COUNT(cm.user_id), c.created_at
		 FROM classes c
		 LEFT JOIN class_members cm ON c.id = cm.class_id
		 WHERE c.organization_id = $1
		 GROUP BY c.id
		 ORDER BY c.created_at DESC
		 OFFSET $2 LIMIT $3`,
		orgID, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.ClassListItem
	for rows.Next() {
		var c model.ClassListItem
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.Name, &c.Description,
			&c.StudentCount, &c.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, c)
	}
	if items == nil {
		items = []model.ClassListItem{}
	}
	return items, total, nil
}

func (s *ClassStore) Update(ctx context.Context, id string, c *model.Class) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE classes SET name = $1, description = $2, updated_at = NOW() WHERE id = $3`,
		c.Name, c.Description, id)
	return err
}

func (s *ClassStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM classes WHERE id = $1", id)
	return err
}

func (s *ClassStore) AddMember(ctx context.Context, classID, userID, role string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO class_members (class_id, user_id, role)
		 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		classID, userID, role)
	return err
}

func (s *ClassStore) RemoveMember(ctx context.Context, classID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM class_members WHERE class_id = $1 AND user_id = $2`,
		classID, userID)
	return err
}

func (s *ClassStore) GetMembers(ctx context.Context, classID string) ([]model.ClassMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT cm.class_id, cm.user_id, u.username, cm.role, cm.joined_at
		 FROM class_members cm
		 JOIN users u ON cm.user_id = u.id
		 WHERE cm.class_id = $1
		 ORDER BY cm.joined_at`,
		classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []model.ClassMember
	for rows.Next() {
		var m model.ClassMember
		if err := rows.Scan(&m.ClassID, &m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if members == nil {
		members = []model.ClassMember{}
	}
	return members, nil
}

func (s *ClassStore) IsMember(ctx context.Context, classID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM class_members WHERE class_id = $1 AND user_id = $2)`,
		classID, userID).Scan(&exists)
	return exists, err
}

func (s *ClassStore) GetMemberCount(ctx context.Context, classID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM class_members WHERE class_id = $1`,
		classID).Scan(&count)
	return count, err
}
