package model

import "time"

type APIKey struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	KeyHash     string     `json:"-"`
	KeyPreview  string     `json:"key_preview"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	RateLimit   int        `json:"rate_limit"`
	IsActive    bool       `json:"is_active"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type CreateAPIKeyRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RateLimit   int    `json:"rate_limit,omitempty"`
}

type CreateAPIKeyResponse struct {
	APIKey *APIKey `json:"api_key"`
	Secret string  `json:"secret"`
}
