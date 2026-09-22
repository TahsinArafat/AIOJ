package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

// CSRF implements the double-submit cookie pattern for non-Bearer requests.
// Safe methods (GET/HEAD/OPTIONS) pass through and mint a csrf cookie if absent.
// State-changing methods require a matching X-CSRF-Token header when the
// request is not authenticated via Authorization: Bearer (SPA Bearer tokens
// are not sent automatically by browsers, so they are CSRF-immune).
func CSRF(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				if c, err := r.Cookie("csrf"); err != nil || c.Value == "" {
					tok := newCSRFCookie(secret)
					http.SetCookie(w, &http.Cookie{
						Name:     "csrf",
						Value:    tok,
						Path:     "/",
						SameSite: http.SameSiteLaxMode,
						HttpOnly: false, // JS must echo it in X-CSRF-Token
					})
				}
				next.ServeHTTP(w, r)
				return
			}

			// Bearer-authenticated API calls are not vulnerable to CSRF.
			if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("csrf")
			if err != nil || cookie.Value == "" {
				http.Error(w, "csrf cookie missing", http.StatusForbidden)
				return
			}
			header := r.Header.Get("X-CSRF-Token")
			if header == "" || !hmac.Equal([]byte(cookie.Value), []byte(header)) {
				http.Error(w, "csrf token invalid", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// newCSRFCookie returns a random hex token; secret is reserved for future
// signed-cookie hardening and is unused while double-submit equality holds.
func newCSRFCookie(secret string) string {
	_ = secret
	raw := make([]byte, 16)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}
