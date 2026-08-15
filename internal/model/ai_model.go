package model

import "time"

// AIModel represents an AI endpoint configuration managed via the admin panel.
// Multiple models can be registered; the one with the highest priority that
// is enabled is selected automatically for problem generation.
type AIModel struct {
	ID          string    `json:"id"                    db:"id"`
	Name        string    `json:"name"                  db:"name"`
	Endpoint    string    `json:"endpoint"              db:"endpoint"`
	APIKey      string    `json:"api_key,omitempty"     db:"api_key"` // omitted from list; included in GetByID
	ModelName   string    `json:"model_name"            db:"model_name"`
	Enabled     bool      `json:"enabled"               db:"enabled"`
	Priority    int       `json:"priority"              db:"priority"`
	Description string    `json:"description"           db:"description"`
	CreatedAt   time.Time `json:"created_at"            db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"            db:"updated_at"`
}
