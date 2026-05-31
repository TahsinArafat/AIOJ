package vjudge

import (
	"context"
)

type BotState string

const (
	StateIdle    BotState = "idle"
	StateRunning BotState = "running"
	StateError   BotState = "error"
)

type RemoteResult struct {
	RemoteID   string  `json:"remote_id"`
	Verdict    string  `json:"verdict"`
	Status     string  `json:"status"`
	TimeUsed   int     `json:"time_used_ms"`
	MemoryUsed int     `json:"memory_used_kb"`
	Score      float64 `json:"score"`
	Done       bool    `json:"done"`
}

type Bot interface {
	Name() string
	Submit(ctx context.Context, problemID, sourceCode, language string) (string, error)
	Poll(ctx context.Context, remoteSubmissionID string) (*RemoteResult, error)
	State() BotState
	Login(ctx context.Context) (map[string]string, error)
	IsLoggedIn(ctx context.Context) bool
}

type BotConfig struct {
	Username  string            `yaml:"username"`
	Password  string            `yaml:"password"`
	APIKey    string            `yaml:"api_key,omitempty"`
	APISecret string            `yaml:"api_secret,omitempty"`
	BaseURL   string            `yaml:"base_url,omitempty"`
	Cookies   map[string]string `yaml:"cookies,omitempty"`
	UserAgent string            `yaml:"user_agent,omitempty"`
}
