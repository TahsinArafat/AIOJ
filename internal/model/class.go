package model

import "time"

type Class struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	OrgName        string    `json:"org_name,omitempty"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	InviteCode     string    `json:"invite_code"`
	CreatedBy      string    `json:"created_by"`
	CreatorName    string    `json:"creator_name,omitempty"`
	StudentCount   int       `json:"student_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ClassMember struct {
	ClassID  string    `json:"class_id"`
	UserID   string    `json:"user_id"`
	Username string    `json:"username,omitempty"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

type CreateClassRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ClassListItem struct {
	ID           string    `json:"id"`
	OrganizationID string  `json:"organization_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	StudentCount int       `json:"student_count"`
	CreatedAt    time.Time `json:"created_at"`
}
