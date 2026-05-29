package model

import "time"

type Organization struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	CreatorName string    `json:"creator_name,omitempty"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type OrganizationMember struct {
	OrganizationID string    `json:"organization_id"`
	UserID         string    `json:"user_id"`
	Username       string    `json:"username,omitempty"`
	Role           string    `json:"role"`
	JoinedAt       time.Time `json:"joined_at"`
}

type CreateOrganizationRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type OrganizationListItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}
