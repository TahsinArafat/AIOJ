package model

import "time"

type BotAccount struct {
	ID           string            `json:"id"`
	UserID       string            `json:"user_id"`
	Platform     string            `json:"platform"`
	PlatformUser string            `json:"-"`
	PlatformPass string            `json:"-"`
	APIKey       string            `json:"api_key,omitempty"`
	APISecret    string            `json:"api_secret,omitempty"`
	SessionData  map[string]string `json:"session_data,omitempty"`
	Status       string            `json:"status"`
	RateLimitRPS float32           `json:"rate_limit_rps"`
	LastUsedAt   *time.Time        `json:"last_used_at,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

type VJSubmitRequest struct {
	ProblemID       string `json:"problem_id"`
	ProblemRemoteID string `json:"problem_remote_id"`
	RemoteOJ        string `json:"remote_oj"`
	Language        string `json:"language"`
	SourceCode      string `json:"source_code"`
}
