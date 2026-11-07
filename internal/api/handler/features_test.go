package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/feature"
	"github.com/tahsinarafat/aioj/internal/model"
)

type featureStore struct {
	items map[string]model.SystemSetting
}

func newFeatureStore() *featureStore {
	return &featureStore{items: map[string]model.SystemSetting{}}
}

func (s *featureStore) List(context.Context) ([]model.SystemSetting, error) {
	out := make([]model.SystemSetting, 0, len(s.items))
	for _, v := range s.items {
		out = append(out, v)
	}
	return out, nil
}

func (s *featureStore) Get(_ context.Context, key string) (*model.SystemSetting, error) {
	if v, ok := s.items[key]; ok {
		return &v, nil
	}
	return nil, nil
}

func (s *featureStore) Set(_ context.Context, setting *model.SystemSetting) error {
	s.items[setting.Key] = *setting
	return nil
}

func (s *featureStore) Delete(_ context.Context, key string) error {
	delete(s.items, key)
	return nil
}

func withKey(r *http.Request, key string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("key", key)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func newFeaturesHandler() (*FeaturesHandler, *featureStore) {
	store := newFeatureStore()
	resolver := feature.NewResolver(store, nil)
	return NewFeaturesHandler(resolver, store), store
}

func TestFeaturesListReportsEveryFeature(t *testing.T) {
	h, _ := newFeaturesHandler()

	rec := httptest.NewRecorder()
	h.List(rec, httptest.NewRequest(http.MethodGet, "/api/features", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Data []feature.Resolved `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body.Data) != len(feature.All()) {
		t.Fatalf("returned %d features, want %d", len(body.Data), len(feature.All()))
	}
	for _, f := range body.Data {
		// Every feature ships open; the only thing an anonymous caller can be
		// denied is a feature that requires an account.
		if f.RequiresLogin {
			if f.Allowed || f.Reason != feature.ReasonLoginRequired {
				t.Fatalf("feature %q requires login but was allowed for an anonymous caller (%+v)", f.Key, f)
			}
			continue
		}
		if !f.Allowed {
			t.Fatalf("feature %q is denied to an anonymous caller; the registry must ship open", f.Key)
		}
	}
}

func TestFeaturesListResolvesForASignedInUser(t *testing.T) {
	h, _ := newFeaturesHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserContextKey, &auth.Claims{
		UserID: "u1", Username: "user", Role: model.RoleUser,
	}))
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Data []feature.Resolved `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, f := range body.Data {
		if !f.Allowed {
			t.Fatalf("feature %q denied to a signed-in user by default (%+v)", f.Key, f)
		}
	}
}

func TestFeaturesAdminListMarksCustomisedPolicies(t *testing.T) {
	h, store := newFeaturesHandler()
	raw, _ := json.Marshal(feature.Policy{Enabled: false})
	store.items[feature.SettingKey(feature.KeyGym)] = model.SystemSetting{
		Key: feature.SettingKey(feature.KeyGym), Value: raw,
	}

	rec := httptest.NewRecorder()
	h.AdminList(rec, httptest.NewRequest(http.MethodGet, "/api/admin/features", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body struct {
		Data []struct {
			Key        string         `json:"key"`
			Policy     feature.Policy `json:"policy"`
			Customized bool           `json:"customized"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	var found bool
	for _, f := range body.Data {
		if f.Key != feature.KeyGym {
			if f.Customized {
				t.Fatalf("feature %q has no stored policy but is marked customized", f.Key)
			}
			continue
		}
		found = true
		if f.Policy.Enabled {
			t.Fatal("the stored policy must win over the default")
		}
		if !f.Customized {
			t.Fatal("a stored policy that differs from the default must be marked customized")
		}
	}
	if !found {
		t.Fatal("gym missing from the admin listing")
	}
}

func TestFeaturesAdminSetPersistsAndApplies(t *testing.T) {
	h, store := newFeaturesHandler()

	body := `{"enabled":true,"roles":["setter"],"min_rating":1200,"max_rating":0,"message":"setters only"}`
	rec := httptest.NewRecorder()
	req := withKey(httptest.NewRequest(http.MethodPut, "/api/admin/features/gym", strings.NewReader(body)), feature.KeyGym)
	h.AdminSet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	stored, ok := store.items[feature.SettingKey(feature.KeyGym)]
	if !ok {
		t.Fatal("policy was not written to system_settings")
	}
	var policy feature.Policy
	if err := json.Unmarshal(stored.Value, &policy); err != nil {
		t.Fatalf("stored value is not a policy: %v", err)
	}
	if len(policy.Roles) != 1 || policy.Roles[0] != model.RoleSetter {
		t.Fatalf("roles = %v, want [setter]", policy.Roles)
	}
	if policy.MinRating != 1200 {
		t.Fatalf("min_rating = %d, want 1200", policy.MinRating)
	}

	// The write must take effect on this instance immediately, not after the
	// cache TTL — the admin expects the next request to be gated.
	decision := h.resolver.Decide(context.Background(), feature.KeyGym, feature.Principal{UserID: "u", Role: model.RoleUser})
	if decision.Allowed || decision.Reason != feature.ReasonRole {
		t.Fatalf("decision after save = %+v, want a role denial", decision)
	}
}

func TestFeaturesAdminSetRejectsBadInput(t *testing.T) {
	h, _ := newFeaturesHandler()

	tests := []struct {
		name string
		key  string
		body string
		want int
	}{
		{
			name: "unknown feature",
			key:  "not_a_feature",
			body: `{"enabled":false}`,
			want: http.StatusNotFound,
		},
		{
			name: "malformed body",
			key:  feature.KeyGym,
			body: `{"enabled":`,
			want: http.StatusBadRequest,
		},
		{
			name: "unknown role",
			key:  feature.KeyGym,
			body: `{"enabled":true,"roles":["wizard"]}`,
			want: http.StatusBadRequest,
		},
		{
			name: "inverted rating window",
			key:  feature.KeyGym,
			body: `{"enabled":true,"min_rating":2000,"max_rating":1200}`,
			want: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := withKey(httptest.NewRequest(http.MethodPut, "/api/admin/features/"+tc.key, strings.NewReader(tc.body)), tc.key)
			h.AdminSet(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestFeaturesAdminResetRestoresTheDefault(t *testing.T) {
	h, store := newFeaturesHandler()
	raw, _ := json.Marshal(feature.Policy{Enabled: false})
	store.items[feature.SettingKey(feature.KeyGym)] = model.SystemSetting{
		Key: feature.SettingKey(feature.KeyGym), Value: raw,
	}

	rec := httptest.NewRecorder()
	req := withKey(httptest.NewRequest(http.MethodDelete, "/api/admin/features/gym", nil), feature.KeyGym)
	h.AdminReset(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if _, ok := store.items[feature.SettingKey(feature.KeyGym)]; ok {
		t.Fatal("the stored policy must be deleted on reset")
	}
	if d := h.resolver.Decide(context.Background(), feature.KeyGym, feature.Principal{}); !d.Allowed {
		t.Fatalf("feature must be open again after reset, got %+v", d)
	}
}
