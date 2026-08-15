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

	// Configure applies account credentials/settings to the bot before a
	// submission attempt.  Implementations that do not support session-based
	// configuration may treat this as a no-op.
	Configure(cfg BotConfig)

	// SetCookies injects session cookies into the bot's HTTP client.
	// Implementations that do not use cookies may treat this as a no-op.
	SetCookies(cookies map[string]string)

	// FetchLanguages retrieves the list of languages supported by the remote
	// OJ for this bot.  Implementations that cannot enumerate languages
	// should return (nil, nil).
	FetchLanguages(ctx context.Context) ([]RemoteLanguageItem, error)
}

type BotConfig struct {
	Username     string            `yaml:"username"`
	Password     string            `yaml:"password"`
	APIKey       string            `yaml:"api_key,omitempty"`
	APISecret    string            `yaml:"api_secret,omitempty"`
	BaseURL      string            `yaml:"base_url,omitempty"`
	Cookies      map[string]string `yaml:"cookies,omitempty"`
	UserAgent    string            `yaml:"user_agent,omitempty"`
	ProxyURL     string            `yaml:"proxy_url,omitempty"`
	ProxyEnabled bool              `yaml:"proxy_enabled,omitempty"`
}
