package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/auth"
)

func scopeManager() *auth.JWTManager {
	return auth.NewJWTManager(strings.Repeat("k", 32), time.Minute, time.Hour)
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
}

func doRequest(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

// TestAuthMiddleware_QueryTokenOnlyOnAllowlistedPaths is the regression test for
// accepting ?token= on every authenticated route, where it leaks into access
// logs, Referer headers and browser history.
func TestAuthMiddleware_QueryTokenOnlyOnAllowlistedPaths(t *testing.T) {
	mgr := scopeManager()
	token, err := mgr.GenerateAccessToken("u1", "alice", "admin")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	plain := AuthMiddleware(mgr)(okHandler())
	if rec := doRequest(plain, "GET", "/api/submissions?token="+token); rec.Code != http.StatusUnauthorized {
		t.Fatalf("query token on a normal route: status = %d, want 401", rec.Code)
	}

	downloads := AuthMiddleware(mgr, "/api/admin/backups/")(okHandler())
	if rec := doRequest(downloads, "GET", "/api/admin/backups/dump.sql?token="+token); rec.Code != http.StatusOK {
		t.Fatalf("query token on a download route: status = %d, want 200", rec.Code)
	}

	// The allowlist is a prefix check, so it must not leak to a sibling route.
	if rec := doRequest(downloads, "GET", "/api/admin/users?token="+token); rec.Code != http.StatusUnauthorized {
		t.Fatalf("query token outside the prefix: status = %d, want 401", rec.Code)
	}

	// Header auth still works everywhere.
	req := httptest.NewRequest("GET", "/api/submissions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	plain.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("bearer header: status = %d, want 200", rec.Code)
	}
}

// TestAuthMiddleware_MustChangePasswordLocksSession covers the seeded-admin
// flag: such a session may only reach the endpoints needed to set a password.
func TestAuthMiddleware_MustChangePasswordLocksSession(t *testing.T) {
	mgr := scopeManager()
	token, err := mgr.GenerateAccessTokenWithState("u1", "admin", "admin", true)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	h := AuthMiddleware(mgr)(okHandler())

	blocked := []string{
		"/api/submissions",
		"/api/admin/users",
		"/api/problems",
		"/api/contests",
	}
	for _, path := range blocked {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "password_change_required") {
			t.Errorf("%s: body should carry the machine-readable code, got %s", path, rec.Body.String())
		}
	}

	allowed := []string{
		"/api/users/profile/password",
		"/api/users/profile/edit",
		"/api/users/me",
		"/api/auth/logout",
		"/api/auth/2fa/begin",
	}
	for _, path := range allowed {
		req := httptest.NewRequest("PUT", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200 (must stay reachable)", path, rec.Code)
		}
	}
}

func TestAuthMiddleware_NormalSessionUnaffected(t *testing.T) {
	mgr := scopeManager()
	token, _ := mgr.GenerateAccessTokenWithState("u1", "alice", "user", false)

	req := httptest.NewRequest("GET", "/api/submissions", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	AuthMiddleware(mgr)(okHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestMetricsAuth(t *testing.T) {
	mgr := scopeManager()
	adminToken, _ := mgr.GenerateAccessToken("admin-1", "admin", "admin")
	userToken, _ := mgr.GenerateAccessToken("u1", "alice", "user")

	tests := []struct {
		name       string
		static     string
		credential string
		want       int
	}{
		{"anonymous", "scrape-secret", "", http.StatusUnauthorized},
		{"static token", "scrape-secret", "scrape-secret", http.StatusOK},
		{"wrong static token", "scrape-secret", "guess", http.StatusUnauthorized},
		{"admin jwt", "scrape-secret", adminToken, http.StatusOK},
		{"non-admin jwt", "scrape-secret", userToken, http.StatusUnauthorized},
		{"admin jwt with no static token configured", "", adminToken, http.StatusOK},
		{"no static token and no jwt", "", "anything", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := MetricsAuth(tt.static, mgr)(okHandler())
			req := httptest.NewRequest("GET", "/metrics", nil)
			if tt.credential != "" {
				req.Header.Set("Authorization", "Bearer "+tt.credential)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
