package model

import "time"

type PrintRequest struct {
	ID        string    `json:"id"`
	ContestID string    `json:"contest_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Filename  string    `json:"filename"`
	Content   string    `json:"content"`
	Status    string    `json:"status"` // pending, printed, cancelled
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
