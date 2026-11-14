package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/api/handler"
	"github.com/tahsinarafat/aioj/internal/authmethod"
	"github.com/tahsinarafat/aioj/internal/model"
)

// routeStore is a minimal authmethod.SettingsStore for the router smoke test.
type routeStore struct{ row *model.SystemSetting }

func (r *routeStore) Get(_ context.Context, key string) (*model.SystemSetting, error) {
	if r.row == nil || r.row.Key != key {
		return nil, nil
	}
	return r.row, nil
}

func (r *routeStore) Set(_ context.Context, s *model.SystemSetting) error {
	cp := *s
	r.row = &cp
	return nil
}

func (r *routeStore) Delete(_ context.Context, _ string) error {
	r.row = nil
	return nil
}

// TestRouterMountsAuthMethodRoutes is a wiring test, not a behaviour test: it
// proves main.go's handler reaches the mux at the documented paths. A compile
// check cannot catch a route that was never registered, and the symptom would
// be a 404 on the login page's own bootstrap request.
func TestRouterMountsAuthMethodRoutes(t *testing.T) {
	methods := authmethod.NewResolver(&routeStore{}, authmethod.Configured{Google: true, GitHub: false})
	mux := NewRouter(Deps{
		Optional: OptionalDeps{
			AuthMethods: &handler.AuthMethodsHandler{Methods: methods},
		},
	}, nil)

	t.Run("public endpoint is served anonymously", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/auth/methods", nil))

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /api/auth/methods = %d, want 200", rec.Code)
		}
		var body struct {
			Data authmethod.Availability `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// Google is configured, GitHub is not, so only Google may be offered.
		if !body.Data.Google || body.Data.GitHub {
			t.Errorf("availability = %+v, want google only", body.Data)
		}
		if !body.Data.PasswordRegistration {
			t.Error("password registration defaults to on")
		}
	})

	t.Run("public endpoint is not cacheable", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/auth/methods", nil))
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("Cache-Control = %q, want no-store", got)
		}
	})

	// The admin routes must exist and must be closed to anonymous callers. A
	// 404 would mean the path was never mounted; 401 (GET, no credentials) and
	// 403 (PUT, rejected by the CSRF check before auth even runs) both prove
	// the request reached the mux and was refused.
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{"GET", "/api/admin/auth-methods", http.StatusUnauthorized},
		{"PUT", "/api/admin/auth-methods", http.StatusForbidden},
	} {
		t.Run(tc.method+" admin route is closed to anonymous callers", func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"google":true}`))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("%s %s = %d, want %d", tc.method, tc.path, rec.Code, tc.want)
			}
		})
	}
}
