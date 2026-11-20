package model

import "time"

type BotAccount struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	Platform     string `json:"platform"`
	PlatformUser string `json:"platform_user"`
	// Every credential is json:"-" — it must not leave the process. The API
	// used to serialize APIKey and APISecret, so the admin console received
	// working secrets it never needed, and a pre-filled input put them in the
	// page source. Masking only PlatformPass had made the two look safe.
	//
	// Has* carry what the UI actually needs: whether a secret is configured, so
	// the form can say "leave blank to keep" instead of showing the value.
	PlatformPass string `json:"-"`
	APIKey       string `json:"-"`
	APISecret    string `json:"-"`
	HasAPIKey    bool   `json:"has_api_key"`
	HasAPISecret bool   `json:"has_api_secret"`
	// SessionData is the harvested cookie jar — a bearer credential for the
	// remote judge, not display data. It used to be serialized in full, so the
	// console received a working session with every bot account. HasSessionData
	// is what the UI renders.
	SessionData         map[string]string `json:"-"`
	HasSessionData      bool              `json:"has_session_data"`
	Status              string            `json:"status"`
	RateLimitRPS        float32           `json:"rate_limit_rps"`
	LastUsedAt          *time.Time        `json:"last_used_at,omitempty"`
	ConsecutiveFailures int               `json:"consecutive_failures"`
	LastErrorAt         *time.Time        `json:"last_error_at,omitempty"`
	LastPollAt          *time.Time        `json:"last_poll_at,omitempty"`
	ProxyURL            string            `json:"proxy_url"`
	ProxyEnabled        bool              `json:"proxy_enabled"`
	CreatedAt           time.Time         `json:"created_at"`
}

type VJSubmitRequest struct {
	ProblemID       string `json:"problem_id"`
	ProblemRemoteID string `json:"problem_remote_id"`
	RemoteOJ        string `json:"remote_oj"`
	Language        string `json:"language"`
	SourceCode      string `json:"source_code"`
}
