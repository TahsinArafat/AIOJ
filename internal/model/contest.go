package model

import "time"

type Contest struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     time.Time  `json:"end_time"`
	FreezeTime  *time.Time `json:"freeze_time,omitempty"`
	Password    string     `json:"-"`
	Visible     bool       `json:"visible"`
	Description string     `json:"description,omitempty"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
}

type ContestProblem struct {
	ContestID string `json:"contest_id"`
	ProblemID string `json:"problem_id"`
	Index     string `json:"index"`
	Score     int    `json:"score"`
	SortOrder int    `json:"sort_order"`
}

type ProblemResult struct {
	Solved   bool `json:"solved"`
	Attempts int  `json:"attempts"`
	Time     int  `json:"time"`
	Score    int  `json:"score"`
	Pending  int  `json:"pending"`
}

type ContestRankEntry struct {
	UserID       string                   `json:"user_id"`
	Username     string                   `json:"username"`
	Problems     map[string]ProblemResult `json:"problems"`
	TotalSolved  int                      `json:"total_solved"`
	TotalPenalty int                      `json:"total_penalty"`
	TotalScore   int                      `json:"total_score"`
	LastACTime   *time.Time               `json:"last_ac_time,omitempty"`
}

type CreateContestRequest struct {
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     time.Time  `json:"end_time"`
	FreezeTime  *time.Time `json:"freeze_time,omitempty"`
	Password    string     `json:"password,omitempty"`
	Description string     `json:"description,omitempty"`
	ProblemIDs  []string   `json:"problem_ids"`
}
