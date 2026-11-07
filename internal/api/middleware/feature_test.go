package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/feature"
	"github.com/tahsinarafat/aioj/internal/model"
)

type stubSettings struct{ items map[string]model.SystemSetting }

func (s *stubSettings) List(context.Context) ([]model.SystemSetting, error) {
	out := make([]model.SystemSetting, 0, len(s.items))
	for _, v := range s.items {
		out = append(out, v)
	}
	return out, nil
}

func (s *stubSettings) Get(_ context.Context, key string) (*model.SystemSetting, error) {
	if v, ok := s.items[key]; ok {
		return &v, nil
	}
	return nil, nil
}

func (s *stubSettings) Set(_ context.Context, setting *model.SystemSetting) error {
	s.items[setting.Key] = *setting
	return nil
}

func (s *stubSettings) Delete(_ context.Context, key string) error {
	delete(s.items, key)
	return nil
}

func (s *stubSettings) put(t *testing.T, key string, p feature.Policy) {
	t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	s.items[feature.SettingKey(key)] = model.SystemSetting{Key: feature.SettingKey(key), Value: raw}
}

func newGate(t *testing.T, key string, configure func(*stubSettings)) (http.Handler, *auth.JWTManager) {
	t.Helper()
	store := &stubSettings{items: map[string]model.SystemSetting{}}
	if configure != nil {
		configure(store)
	}
	resolver := feature.NewResolver(store, nil)
	jwt := auth.NewJWTManager("test-secret-that-is-long-enough-for-hmac", time.Minute, time.Hour)

	handler := FeatureGate(jwt, resolver, key)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	return handler, jwt
}

func do(h http.Handler, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/gym", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestFeatureGateAllowsByDefault(t *testing.T) {
	h, _ := newGate(t, feature.KeyGym, nil)
	if rec := do(h, ""); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (an untouched feature must stay open)", rec.Code)
	}
}

func TestFeatureGateBlocksDisabledFeature(t *testing.T) {
	h, jwt := newGate(t, feature.KeyGym, func(s *stubSettings) {
		s.put(t, feature.KeyGym, feature.Policy{Enabled: false, Message: "back on Monday"})
	})

	token, err := jwt.GenerateAccessToken("admin-1", "admin", model.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}

	// The master switch is absolute: even an admin is blocked, otherwise
	// "disabled" would be untrue for the one person who can see it.
	for _, tok := range []string{"", token} {
		rec := do(h, tok)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}

		var body struct {
			Feature string `json:"feature"`
			Reason  string `json:"reason"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("body is not JSON: %v (%s)", err, rec.Body.String())
		}
		if body.Feature != feature.KeyGym {
			t.Fatalf("feature = %q, want %q", body.Feature, feature.KeyGym)
		}
		if body.Reason != feature.ReasonDisabled {
			t.Fatalf("reason = %q, want %q", body.Reason, feature.ReasonDisabled)
		}
		if body.Message != "back on Monday" {
			t.Fatalf("message = %q, want the admin message", body.Message)
		}
	}
}

func TestFeatureGateResolvesTheCallerFromTheToken(t *testing.T) {
	// The gate must read the bearer token itself: these route groups mix
	// public and authenticated handlers, so no auth middleware is guaranteed
	// to have run. Without this, a role gate would deny signed-in users too.
	h, jwt := newGate(t, feature.KeyGym, func(s *stubSettings) {
		s.put(t, feature.KeyGym, feature.Policy{Enabled: true, Roles: []string{model.RoleSetter}})
	})

	anon := do(h, "")
	if anon.Code != http.StatusForbidden {
		t.Fatalf("anonymous status = %d, want 403", anon.Code)
	}
	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(anon.Body.Bytes(), &body)
	if body.Reason != feature.ReasonLoginRequired {
		t.Fatalf("anonymous reason = %q, want %q", body.Reason, feature.ReasonLoginRequired)
	}

	userToken, _ := jwt.GenerateAccessToken("u1", "user", model.RoleUser)
	if rec := do(h, userToken); rec.Code != http.StatusForbidden {
		t.Fatalf("plain user status = %d, want 403", rec.Code)
	}

	setterToken, _ := jwt.GenerateAccessToken("s1", "setter", model.RoleSetter)
	if rec := do(h, setterToken); rec.Code != http.StatusOK {
		t.Fatalf("setter status = %d, want 200", rec.Code)
	}
}

func TestFeatureGateIgnoresAnInvalidToken(t *testing.T) {
	h, _ := newGate(t, feature.KeyGym, nil)
	// A garbage token must be treated as anonymous rather than 500 or 401:
	// the gate is not the authentication boundary.
	if rec := do(h, "not-a-jwt"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestFeatureGateIsTransparentForUnknownKeys(t *testing.T) {
	h, _ := newGate(t, "not_a_feature", nil)
	if rec := do(h, ""); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (a typo in a route must not lock a surface out)", rec.Code)
	}
}
