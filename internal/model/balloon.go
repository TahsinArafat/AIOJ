package model

import "time"

type BalloonRequest struct {
	ID           string    `json:"id"`
	ContestID    string    `json:"contest_id"`
	SubmissionID string    `json:"submission_id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username"`
	ProblemID    string    `json:"problem_id"`
	ProblemIndex string    `json:"problem_index"`
	Color        string    `json:"color"`
	Dispatched   bool      `json:"dispatched"`
	CreatedAt    time.Time `json:"created_at"`
}
