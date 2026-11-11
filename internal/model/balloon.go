package model

import "time"

type BalloonRequest struct {
	ID           string `json:"id"`
	ContestID    string `json:"contest_id"`
	SubmissionID string `json:"submission_id"`
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	// TeamName/Institution are set when the solver is a generated onsite
	// account. A runner carrying a balloon to "team_1a2b3c4d" cannot find the
	// team that way, so the name entered in the Onsite Teams panel wins.
	TeamName     string    `json:"team_name,omitempty"`
	Institution  string    `json:"institution,omitempty"`
	ProblemID    string    `json:"problem_id"`
	ProblemIndex string    `json:"problem_index"`
	Color        string    `json:"color"`
	Dispatched   bool      `json:"dispatched"`
	CreatedAt    time.Time `json:"created_at"`
}
