package model

import "time"

type Editorial struct {
	ID               string    `json:"id"`
	ProblemID        string    `json:"problem_id"`
	ProblemTitle     string    `json:"problem_title,omitempty"`
	ContestID        *string   `json:"contest_id,omitempty"`
	UserID           string    `json:"user_id"`
	Username         string    `json:"username,omitempty"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	SolutionCode     string    `json:"solution_code,omitempty"`
	SolutionLanguage string    `json:"solution_language,omitempty"`
	Approach         string    `json:"approach,omitempty"`
	TimeComplexity   string    `json:"time_complexity,omitempty"`
	SpaceComplexity  string    `json:"space_complexity,omitempty"`
	IsOfficial       bool      `json:"is_official"`
	Upvotes          int       `json:"upvotes"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CreateEditorialRequest struct {
	ProblemID        string `json:"problem_id"`
	ContestID        string `json:"contest_id,omitempty"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	SolutionCode     string `json:"solution_code,omitempty"`
	SolutionLanguage string `json:"solution_language,omitempty"`
	Approach         string `json:"approach,omitempty"`
	TimeComplexity   string `json:"time_complexity,omitempty"`
	SpaceComplexity  string `json:"space_complexity,omitempty"`
}
