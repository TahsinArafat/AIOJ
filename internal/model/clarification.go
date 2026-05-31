package model

import "time"

type Clarification struct {
	ID        string    `json:"id"`
	ContestID string    `json:"contest_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username,omitempty"`
	ProblemID *string   `json:"problem_id,omitempty"`
	ProblemIdx *string  `json:"problem_idx,omitempty"`
	Question  string    `json:"question"`
	Answer    *string   `json:"answer,omitempty"`
	IsPublic  bool      `json:"is_public"`
	AnsweredBy *string  `json:"answered_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ContestNotice struct {
	ID        string    `json:"id"`
	ContestID string    `json:"contest_id"`
	Content   string    `json:"content"`
	CreatedBy string    `json:"created_by"`
	Username  string    `json:"username,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
