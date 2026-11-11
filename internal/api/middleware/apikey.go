package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * API-key authentication.
 *
 * Keys could be minted, listed and revoked from the day the feature shipped, and
 * they authenticated nothing: nothing in the codebase read X-API-Key, and
 * APIKeyStore.GetByHash / UpdateLastUsed / IncrementRequestCount all had zero
 * callers. This middleware is the missing verifier.
 *
 * It is deliberately an ALTERNATIVE to the JWT middleware rather than a
 * replacement: it populates the same claims context key, so every downstream
 * handler works unchanged, but it is mounted only on the /api/v1 group. Browser
 * routes keep using cookies and JWTs.
 */

// APIKeyVerifier is the slice of the key store this middleware needs.
type APIKeyVerifier interface {
	GetByHash(ctx context.Context, keyHash string) (*model.APIKey, error)
	UpdateLastUsed(ctx context.Context, id string) error
	IncrementRequestCount(ctx context.Context, keyID string, windowStart time.Time) error
	GetRequestCount(ctx context.Context, keyID string, windowStart time.Time) (int, error)
}

// APIKeyUserLookup resolves the owning user, because the claims a handler needs
// (role, username) live on the user row, not on the key.
type APIKeyUserLookup interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
}

// APIKeyAuth authenticates a request by its X-API-Key header.
//
// On success it stores the same *auth.Claims the JWT middleware would, so
// handlers and the feature gate work without knowing which scheme was used.
func APIKeyAuth(keys APIKeyVerifier, users APIKeyUserLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := strings.TrimSpace(r.Header.Get("X-API-Key"))
			if raw == "" {
				// No key presented: let the next layer decide. This middleware is
				// mounted where a key is required, so that means unauthorized.
				http.Error(w, "unauthorized: X-API-Key required", http.StatusUnauthorized)
				return
			}
			// Every issued key carries this prefix. Rejecting anything else here
			// avoids a database lookup on junk input.
			if !strings.HasPrefix(raw, model.APIKeyPrefix) {
				http.Error(w, "unauthorized: malformed API key", http.StatusUnauthorized)
				return
			}

			key, err := keys.GetByHash(r.Context(), model.HashAPIKey(raw))
			if err != nil {
				slog.Error("apikey: lookup failed", "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if key == nil {
				http.Error(w, "unauthorized: invalid API key", http.StatusUnauthorized)
				return
			}

			// Enforce the per-key budget. A failure to read the counter must not
			// open the gate: an unlimited key is exactly what the limit prevents.
			if key.RateLimit > 0 {
				window := time.Now().UTC().Truncate(time.Minute)
				count, err := keys.GetRequestCount(r.Context(), key.ID, window)
				if err != nil {
					slog.Error("apikey: rate counter failed", "key_id", key.ID, "error", err)
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				if count >= key.RateLimit {
					w.Header().Set("Retry-After", "60")
					http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
					return
				}
				if err := keys.IncrementRequestCount(r.Context(), key.ID, window); err != nil {
					// Non-fatal: the request itself is legitimate, and failing it
					// over a counter write would be worse than under-counting.
					slog.Warn("apikey: increment failed", "key_id", key.ID, "error", err)
				}
			}

			user, err := users.GetByID(r.Context(), key.UserID)
			if err != nil || user == nil {
				slog.Error("apikey: owner lookup failed", "key_id", key.ID, "error", err)
				http.Error(w, "unauthorized: invalid API key", http.StatusUnauthorized)
				return
			}
			// Best-effort bookkeeping, after the request is already authorized.
			if err := keys.UpdateLastUsed(r.Context(), key.ID); err != nil {
				slog.Warn("apikey: last_used update failed", "key_id", key.ID, "error", err)
			}

			claims := &auth.Claims{
				UserID:   user.ID,
				Username: user.Username,
				Role:     user.Role,
				// An API key is not a JWT and carries no refresh/rotation state, so
				// it is marked as an access token to satisfy downstream checks.
				Type: auth.TokenTypeAccess,
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), UserContextKey, claims)))
		})
	}
}
