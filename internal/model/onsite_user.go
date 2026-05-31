package model

import "time"

type OnsiteBatchUser struct {
	ID           string    `json:"id"`
	ContestID    string    `json:"contest_id"`
	TeamName     string    `json:"team_name"`
	Institution  string    `json:"institution,omitempty"`
	Username     string    `json:"username"`
	Password     string    `json:"password,omitempty"`
	PasswordHash string    `json:"-"`
	IsUsed       bool      `json:"is_used"`
	UsedBy       *string   `json:"used_by,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type BatchUserRequest struct {
	TeamName    string `json:"team_name"`
	Institution string `json:"institution,omitempty"`
}
