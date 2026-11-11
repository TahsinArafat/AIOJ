package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/tahsinarafat/aioj/internal/auth"
)

type contextKey string

const UserContextKey contextKey = "user"

// bearerToken returns the Authorization: Bearer credential, if any.
func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

// allowsQueryToken reports whether ?token= may stand in for the header on this
// request. Only browser navigations that cannot set headers (file downloads)
// may pass prefixes; on every other route a query token would leak into access
// logs, Referer headers and browser history.
func allowsQueryToken(r *http.Request, prefixes []string) bool {
	for _, p := range prefixes {
		if p != "" && strings.HasPrefix(r.URL.Path, p) {
			return true
		}
	}
	return false
}

// AuthMiddleware authenticates from the Authorization header.
//
// queryTokenPrefixes opt specific URL prefixes into accepting ?token= as well.
// Pass none and the token can only travel in a header.
func AuthMiddleware(jwtManager *auth.JWTManager, queryTokenPrefixes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" && allowsQueryToken(r, queryTokenPrefixes) {
				token = r.URL.Query().Get("token")
			}

			if token == "" {
				http.Error(w, "missing auth token", http.StatusUnauthorized)
				return
			}
			claims, err := jwtManager.ValidateToken(token)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			// A user flagged for a mandatory password change (the seeded default
			// admin) may only reach the endpoints needed to complete it.
			if claims.MustChangePassword && !passwordChangeAllowed(r.URL.Path) {
				writePasswordChangeRequired(w)
				return
			}
			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// passwordChangeAllowed lists the routes a flagged user still needs: read their
// own profile, change the password, manage 2FA, and sign out.
func passwordChangeAllowed(path string) bool {
	switch {
	case path == "/api/auth/logout",
		path == "/api/auth/refresh",
		path == "/api/users/profile/edit",
		path == "/api/users/profile/password",
		path == "/api/users/me",
		strings.HasPrefix(path, "/api/auth/2fa/"):
		return true
	}
	return false
}

func writePasswordChangeRequired(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"password change required","code":"password_change_required"}`))
}

// OptionalAuthMiddleware attaches claims when a credential is present and lets
// anonymous requests through otherwise.
//
// queryTokenPrefixes works exactly as it does on AuthMiddleware: on the listed
// path prefixes a ?token= query parameter may stand in for the Authorization
// header, because a browser navigation cannot set one. Without this, a download
// that is public for some callers and private for others has no way to
// authenticate the private case — a setter could not fetch their own draft's
// booklet, which is what a plain <a href> on a gated endpoint amounts to.
//
// The same caution applies: a query token leaks into access logs, Referer and
// browser history, so only file-download prefixes may opt in.
func OptionalAuthMiddleware(jwtManager *auth.JWTManager, queryTokenPrefixes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" && allowsQueryToken(r, queryTokenPrefixes) {
				token = r.URL.Query().Get("token")
			}
			if token != "" {
				if claims, err := jwtManager.ValidateToken(token); err == nil {
					ctx := context.WithValue(r.Context(), UserContextKey, claims)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func GetUserClaims(r *http.Request) *auth.Claims {
	c, _ := r.Context().Value(UserContextKey).(*auth.Claims)
	return c
}

// MetricsAuth protects the Prometheus endpoint. It accepts a static scrape
// token (METRICS_TOKEN) or an admin access token; with no token configured only
// admins may scrape. Metrics used to be world-readable, exposing usernames,
// endpoint inventory and traffic shape.
func MetricsAuth(staticToken string, jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r)
			if token == "" {
				// Prometheus sends the credential in `authorization`; accept the
				// same value from ?token= for manual curl use on a private host.
				token = r.URL.Query().Get("token")
			}
			if token == "" {
				http.Error(w, "missing metrics token", http.StatusUnauthorized)
				return
			}
			if staticToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(staticToken)) == 1 {
				next.ServeHTTP(w, r)
				return
			}
			if jwtManager != nil {
				if claims, err := jwtManager.ValidateToken(token); err == nil && claims.Role == "admin" {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "invalid metrics token", http.StatusUnauthorized)
		})
	}
}
