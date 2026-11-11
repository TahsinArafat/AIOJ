package model

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

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

// APIKeyPrefix is the marker every issued key starts with. A caller presenting
// something else is not using an API key at all, so the middleware can reject it
// without a database round-trip.
const APIKeyPrefix = "aioj_"

// HashAPIKey is the digest stored in api_keys.key_hash. SHA-256 is used (not
// bcrypt) because the key is a 256-bit random token, not a human-chosen password:
// there is nothing to brute-force, and verification happens on every request.
//
// It lives here, next to the model, so the issuer (GenerateAPIKey) and the
// verifier (the auth middleware) cannot drift apart — they did not exist
// together before, which is why GetByHash had no caller.
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
