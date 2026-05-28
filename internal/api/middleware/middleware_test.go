package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/auth"
)

func TestAuthMiddleware(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-for-testing", time.Hour, 24*time.Hour)
	token, err := jwtMgr.GenerateAccessToken("user-1", "testuser", "user")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	tests := []struct {
		name       string
		headerKey  string
		headerVal  string
		wantStatus int
	}{
		{"valid token", "Authorization", "Bearer " + token, http.StatusOK},
		{"missing header", "", "", http.StatusUnauthorized},
		{"malformed bearer", "Authorization", token, http.StatusUnauthorized},
		{"empty token", "Authorization", "Bearer ", http.StatusUnauthorized},
		{"invalid token", "Authorization", "Bearer invalid.token.here", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				claims := GetUserClaims(r)
				if claims == nil {
					t.Error("expected claims in context")
					return
				}
				w.WriteHeader(http.StatusOK)
			})

			middleware := AuthMiddleware(jwtMgr)(handler)
			req := httptest.NewRequest("GET", "/", nil)
			if tt.headerKey != "" {
				req.Header.Set(tt.headerKey, tt.headerVal)
			}

			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestOptionalAuthMiddleware(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key", time.Hour, 24*time.Hour)
	token, _ := jwtMgr.GenerateAccessToken("user-1", "testuser", "user")

	tests := []struct {
		name        string
		headerVal   string
		wantClaims  bool
		wantStatus  int
	}{
		{"with valid token", "Bearer " + token, true, http.StatusOK},
		{"without token", "", false, http.StatusOK},
		{"with invalid token", "Bearer bad.token", false, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				claims := GetUserClaims(r)
				if tt.wantClaims && claims == nil {
					t.Error("expected claims but got nil")
				}
				if !tt.wantClaims && claims != nil {
					t.Error("expected no claims but got some")
				}
				w.WriteHeader(http.StatusOK)
			})

			mid := OptionalAuthMiddleware(jwtMgr)(handler)
			req := httptest.NewRequest("GET", "/", nil)
			if tt.headerVal != "" {
				req.Header.Set("Authorization", tt.headerVal)
			}

			rec := httptest.NewRecorder()
			mid.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key", time.Hour, 24*time.Hour)

	tests := []struct {
		name        string
		userRole    string
		allowedRole string
		wantStatus  int
	}{
		{"matching role", "admin", "admin", http.StatusOK},
		{"non-matching role", "user", "admin", http.StatusForbidden},
		{"multiple allowed, matching", "admin", "admin", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			token, _ := jwtMgr.GenerateAccessToken("user-1", "testuser", tt.userRole)
			mid := RequireRole(tt.allowedRole)(handler)

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req = req.WithContext(context.WithValue(req.Context(), UserContextKey, &auth.Claims{
				UserID: "user-1", Username: "testuser", Role: tt.userRole,
			}))

			rec := httptest.NewRecorder()
			mid.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestRequireRole_NoClaims(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mid := RequireRole("admin")(handler)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	mid.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d (unauthorized when no claims)", rec.Code, http.StatusUnauthorized)
	}
}

func TestGetUserClaims(t *testing.T) {
	t.Run("claims exist", func(t *testing.T) {
		expected := &auth.Claims{UserID: "u1", Username: "alice", Role: "admin"}
		ctx := context.WithValue(context.Background(), UserContextKey, expected)
		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)

		got := GetUserClaims(req)
		if got == nil {
			t.Fatal("expected claims")
		}
		if got.UserID != expected.UserID {
			t.Errorf("UserID = %s, want %s", got.UserID, expected.UserID)
		}
	})

	t.Run("no claims", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		got := GetUserClaims(req)
		if got != nil {
			t.Error("expected nil when no claims in context")
		}
	})

	t.Run("wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), UserContextKey, "not-claims")
		req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
		got := GetUserClaims(req)
		if got != nil {
			t.Error("expected nil when wrong type in context")
		}
	})
}
