package model

import "time"

type Group struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	IsPublic    bool       `json:"is_public"`
	MaxMembers  *int       `json:"max_members,omitempty"`
	CreatedBy   string     `json:"created_by"`
	CreatorName string     `json:"creator_name,omitempty"`
	MemberCount int        `json:"member_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type GroupMember struct {
	GroupID    string    `json:"group_id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username,omitempty"`
	Role       string    `json:"role"`
	JoinedAt   time.Time `json:"joined_at"`
}

type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	MaxMembers  *int   `json:"max_members,omitempty"`
}

type GroupListItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}
