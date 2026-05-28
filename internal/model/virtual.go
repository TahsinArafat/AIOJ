package model

import "time"

type VirtualContest struct {
	ID                string     `json:"id"`
	OriginalContestID string     `json:"original_contest_id"`
	UserID            string     `json:"user_id"`
	Username          string     `json:"username,omitempty"`
	ContestTitle      string     `json:"contest_title,omitempty"`
	StartedAt         time.Time  `json:"started_at"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	DurationMinutes   int        `json:"duration_minutes"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
}

type VirtualStatus struct {
	IsActive      bool       `json:"is_active"`
	VirtualID     string     `json:"virtual_id,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	EndsAt        *time.Time `json:"ends_at,omitempty"`
	RemainingMins int        `json:"remaining_minutes"`
}
