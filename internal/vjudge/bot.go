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
}

type BotConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	BaseURL  string `yaml:"base_url,omitempty"`
}
