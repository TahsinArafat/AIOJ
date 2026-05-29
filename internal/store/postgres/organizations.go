package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type OrganizationStore struct {
	db *sql.DB
}

func NewOrganizationStore(db *sql.DB) *OrganizationStore {
	return &OrganizationStore{db: db}
}

func (s *OrganizationStore) Create(ctx context.Context, o *model.Organization) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx,
		`INSERT INTO organizations (name, description, created_by)
		 VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`,
		o.Name, o.Description, o.CreatedBy,
	).Scan(&o.ID, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, 'owner')`,
		o.ID, o.CreatedBy)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *OrganizationStore) GetByID(ctx context.Context, id string) (*model.Organization, error) {
	var o model.Organization
	err := s.db.QueryRowContext(ctx,
		`SELECT o.id, o.name, o.description, o.created_by,
		        u.username, COUNT(om.user_id), o.created_at, o.updated_at
		 FROM organizations o
		 JOIN users u ON o.created_by = u.id
		 LEFT JOIN organization_members om ON o.id = om.organization_id
		 WHERE o.id = $1
		 GROUP BY o.id, u.username`,
		id).Scan(&o.ID, &o.Name, &o.Description, &o.CreatedBy,
		&o.CreatorName, &o.MemberCount, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *OrganizationStore) List(ctx context.Context, offset, limit int) ([]model.OrganizationListItem, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM organizations").Scan(&total)

	rows, err := s.db.QueryContext(ctx,
		`SELECT o.id, o.name, o.description, COUNT(om.user_id), o.created_at
		 FROM organizations o
		 LEFT JOIN organization_members om ON o.id = om.organization_id
		 GROUP BY o.id
		 ORDER BY o.created_at DESC
		 OFFSET $1 LIMIT $2`,
		offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.OrganizationListItem
	for rows.Next() {
		var o model.OrganizationListItem
		if err := rows.Scan(&o.ID, &o.Name, &o.Description, &o.MemberCount, &o.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, o)
	}
	if items == nil {
		items = []model.OrganizationListItem{}
	}
	return items, total, nil
}

func (s *OrganizationStore) Update(ctx context.Context, id string, o *model.Organization) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE organizations SET name = $1, description = $2, updated_at = NOW() WHERE id = $3`,
		o.Name, o.Description, id)
	return err
}

func (s *OrganizationStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM organizations WHERE id = $1", id)
	return err
}

func (s *OrganizationStore) AddMember(ctx context.Context, orgID, userID, role string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO organization_members (organization_id, user_id, role)
		 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		orgID, userID, role)
	return err
}

func (s *OrganizationStore) RemoveMember(ctx context.Context, orgID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID)
	return err
}

func (s *OrganizationStore) GetMembers(ctx context.Context, orgID string) ([]model.OrganizationMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT om.organization_id, om.user_id, u.username, om.role, om.joined_at
		 FROM organization_members om
		 JOIN users u ON om.user_id = u.id
		 WHERE om.organization_id = $1
		 ORDER BY om.joined_at`,
		orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []model.OrganizationMember
	for rows.Next() {
		var m model.OrganizationMember
		if err := rows.Scan(&m.OrganizationID, &m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if members == nil {
		members = []model.OrganizationMember{}
	}
	return members, nil
}

func (s *OrganizationStore) IsMember(ctx context.Context, orgID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM organization_members WHERE organization_id = $1 AND user_id = $2)`,
		orgID, userID).Scan(&exists)
	return exists, err
}

func (s *OrganizationStore) GetMemberRole(ctx context.Context, orgID, userID string) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx,
		`SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return role, err
}

func (s *OrganizationStore) GetMemberCount(ctx context.Context, orgID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM organization_members WHERE organization_id = $1`,
		orgID).Scan(&count)
	return count, err
}

func (s *OrganizationStore) ListByUser(ctx context.Context, userID string) ([]model.OrganizationListItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT o.id, o.name, o.description, COUNT(om2.user_id), o.created_at
		 FROM organizations o
		 JOIN organization_members om ON o.id = om.organization_id
		 LEFT JOIN organization_members om2 ON o.id = om2.organization_id
		 WHERE om.user_id = $1
		 GROUP BY o.id
		 ORDER BY o.name`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.OrganizationListItem
	for rows.Next() {
		var o model.OrganizationListItem
		if err := rows.Scan(&o.ID, &o.Name, &o.Description, &o.MemberCount, &o.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	if items == nil {
		items = []model.OrganizationListItem{}
	}
	return items, nil
}
