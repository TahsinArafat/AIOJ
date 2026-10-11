package model

import "time"

// AIModel represents an AI endpoint configuration managed via the admin panel.
// Multiple models can be registered; the one with the highest priority that
// is enabled is selected automatically for problem generation.
type AIModel struct {
	ID                    string         `json:"id"                    db:"id"`
	Name                  string         `json:"name"                  db:"name"`
	Endpoint              string         `json:"endpoint"              db:"endpoint"`
	APIKey                string         `json:"api_key,omitempty"     db:"api_key"` // omitted from list; included in GetByID
	HasAPIKey             bool           `json:"has_api_key" db:"-"`
	ModelName             string         `json:"model_name"            db:"model_name"`
	Enabled               bool           `json:"enabled"               db:"enabled"`
	Priority              int            `json:"priority"              db:"priority"`
	Description           string         `json:"description"           db:"description"`
	AllowPrivateNetwork   bool           `json:"allow_private_network" db:"allow_private_network"`
	StructuredOutputMode  string         `json:"structured_output_mode" db:"structured_output_mode"`
	RequestTimeoutSeconds int            `json:"request_timeout_seconds" db:"request_timeout_seconds"`
	MaxOutputTokens       int            `json:"max_output_tokens" db:"max_output_tokens"`
	Capabilities          map[string]any `json:"capabilities,omitempty" db:"-"`
	LastTestStatus        string         `json:"last_test_status,omitempty" db:"last_test_status"`
	LastTestedAt          *time.Time     `json:"last_tested_at,omitempty" db:"last_tested_at"`
	CreatedAt             time.Time      `json:"created_at"            db:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"            db:"updated_at"`
}
