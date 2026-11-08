package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/api/handler"
)

// TestRouterMountsOAuthSignupRoutes pins the three OAuth routes a first-time
// sign-in depends on.
//
// The static "username-available" and "complete" paths sit beside the
// "{provider}" parameter, so a routing mistake here would either shadow the
// callback or leave the username step unreachable — and the browser would be
// stranded on /oauth/complete with a ticket it could never spend.
func TestRouterMountsOAuthSignupRoutes(t *testing.T) {
	mux := NewRouter(Deps{
		OAuthCallback: &handler.OAuthCallbackHandler{
			StateSecret: []byte("router-test-secret"),
			StateTTL:    time.Minute,
			RespondJSON: true,
		},
	}, nil)

	// The SPA needs the double-submit cookie before an unauthenticated POST.
	prime := httptest.NewRecorder()
	mux.ServeHTTP(prime, httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil))
	var csrf *http.Cookie
	for _, c := range prime.Result().Cookies() {
		if c.Name == "csrf" {
			csrf = c
		}
	}
	if csrf == nil {
		t.Fatal("GET /api/auth/csrf did not mint a cookie")
	}

	t.Run("callback still routes past the static siblings", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/oauth/fake/callback", nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("GET callback = %d, want 400 from the handler", rec.Code)
		}
	})

	t.Run("username availability is reachable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
			"/api/auth/oauth/username-available?signup_token=nope&username=alice", nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("GET username-available = %d, want 400 for an invalid ticket", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "ticket") {
			t.Errorf("expected the handler's JSON error, got %s", rec.Body.String())
		}
	})

	t.Run("completing a sign-up reaches the handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/oauth/complete",
			strings.NewReader(`{"signup_token":"nope","username":"alice"}`))
		req.AddCookie(csrf)
		req.Header.Set("X-CSRF-Token", csrf.Value)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("POST complete = %d, want 400 for an invalid ticket (%s)", rec.Code, rec.Body.String())
		}
	})
}
