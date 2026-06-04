package model

import "time"

type Team struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	Rating       int       `json:"rating"`
	MaxRating    int       `json:"max_rating"`
	ContestCount int       `json:"contest_count"`
	MemberCount  int       `json:"member_count"`
	CreatedBy    string    `json:"created_by"`
	CreatorName  string    `json:"creator_name,omitempty"`
	IsPublic     bool      `json:"is_public"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TeamMember struct {
	TeamID    string    `json:"team_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username,omitempty"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

type TeamContest struct {
	ID             string    `json:"id"`
	TeamID         string    `json:"team_id"`
	ContestID      string    `json:"contest_id"`
	ContestTitle   string    `json:"contest_title,omitempty"`
	Rank           *int      `json:"rank,omitempty"`
	Score          int       `json:"score"`
	RatingChange   int       `json:"rating_change"`
	ParticipatedAt time.Time `json:"participated_at"`
}

type CreateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    *bool  `json:"is_public"`
}

type UpdateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatar_url"`
	IsPublic    *bool  `json:"is_public"`
}

type TeamListItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Rating      int       `json:"rating"`
	MemberCount int       `json:"member_count"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
}

type TeamInviteRequest struct {
	Username string `json:"username"`
}

type TeamRespondRequest struct {
	UserID string `json:"user_id"`
	Action string `json:"action"` // "accept", "decline", "approve", "reject"
}

type PendingInvite struct {
	TeamID   string    `json:"team_id"`
	TeamName string    `json:"team_name"`
	Role     string    `json:"role"` // "invited" or "requested"
	JoinedAt time.Time `json:"joined_at"`
}
