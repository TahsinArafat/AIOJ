package model

import "time"

type Hack struct {
	ID                   string     `json:"id"`
	ContestID            string     `json:"contest_id"`
	ProblemID            string     `json:"problem_id"`
	HackerID             string     `json:"hacker_id"`
	HackerUsername       string     `json:"hacker_username,omitempty"`
	DefenderID           string     `json:"defender_id"`
	DefenderUsername     string     `json:"defender_username,omitempty"`
	SubmissionID         string     `json:"submission_id"`
	TestInput            string     `json:"test_input"`
	ExpectedOutput       string     `json:"expected_output,omitempty"`
	ActualOutput         string     `json:"actual_output,omitempty"`
	Status               string     `json:"status"`
	Success              *bool      `json:"success,omitempty"`
	HackerRatingChange   int        `json:"hacker_rating_change"`
	DefenderRatingChange int        `json:"defender_rating_change"`
	CreatedAt            time.Time  `json:"created_at"`
	JudgedAt             *time.Time `json:"judged_at,omitempty"`
}

type HackRequest struct {
	ContestID    string `json:"contest_id"`
	ProblemID    string `json:"problem_id"`
	SubmissionID string `json:"submission_id"`
	TestInput    string `json:"test_input"`
}

type HackResult struct {
	HackID         string `json:"hack_id"`
	Status         string `json:"status"`
	Success        bool   `json:"success"`
	ExpectedOutput string `json:"expected_output,omitempty"`
	ActualOutput   string `json:"actual_output,omitempty"`
	Message        string `json:"message,omitempty"`
}
