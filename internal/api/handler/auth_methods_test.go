package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/authmethod"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/oauth"
	"golang.org/x/oauth2"
)

// methodStore is an in-memory authmethod.SettingsStore.
type methodStore struct {
	row *model.SystemSetting
}

func (m *methodStore) Get(_ context.Context, key string) (*model.SystemSetting, error) {
	if m.row == nil || m.row.Key != key {
		return nil, nil
	}
	return m.row, nil
}

func (m *methodStore) Set(_ context.Context, s *model.SystemSetting) error {
	cp := *s
	m.row = &cp
	return nil
}

func (m *methodStore) Delete(_ context.Context, _ string) error {
	m.row = nil
	return nil
}

func resolverWith(t *testing.T, raw string, configured authmethod.Configured) (*authmethod.Resolver, *methodStore) {
	t.Helper()
	store := &methodStore{}
	if raw != "" {
		store.row = &model.SystemSetting{Key: authmethod.SettingKey, Value: json.RawMessage(raw)}
	}
	return authmethod.NewResolver(store, configured), store
}

func TestRegisterRejectsPasswordRegistrationWhenDisabled(t *testing.T) {
	methods, _ := resolverWith(t, `{"password_registration":false}`, authmethod.Configured{})
	h := &AuthHandler{}
	h.SetAuthMethods(methods)

	req := httptest.NewRequest("POST", "/api/auth/register",
		strings.NewReader(`{"username":"u","email":"u@example.com","password":"correct horse battery staple"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when registration is closed, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "disabled") {
		t.Errorf("the error should say the method is disabled, got %q", rec.Body.String())
	}
}

func TestRegisterWithoutResolverIsUngated(t *testing.T) {
	// A nil resolver must mean "no gate": the handler is built as a literal in
	// several existing tests, and silently failing them would be worse than
	// leaving the gate opt-in. main.go always installs one.
	h := &AuthHandler{}
	req := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Fatal("a nil resolver must not gate registration")
	}
}

func startRequest(provider string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", provider)
	req := httptest.NewRequest("GET", "/api/auth/oauth/"+provider+"/start", nil)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func providerWithClientID(id string) oauth.Provider {
	return &fakeOAuthProvider{cfg: &oauth2.Config{
		ClientID: id, ClientSecret: "sec", RedirectURL: "http://localhost/cb",
		Scopes:   []string{"email"},
		Endpoint: oauth2.Endpoint{AuthURL: "http://provider.example/authorize"},
	}}
}

func TestOAuthStartRefusesUnconfiguredProvider(t *testing.T) {
	// No client id: AuthCodeURL would emit a redirect the provider answers
	// with an error page, so refuse it here with something readable.
	methods, _ := resolverWith(t, "", authmethod.Configured{Google: false})
	h := &OAuthStartHandler{
		Providers:   map[string]oauth.Provider{"google": providerWithClientID("")},
		StateSecret: []byte("s"),
		StateTTL:    5 * time.Minute,
		Methods:     methods,
	}
	rec := httptest.NewRecorder()
	h.Start(rec, startRequest("google"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for an unconfigured provider, got %d", rec.Code)
	}
}

func TestOAuthStartRefusesDisabledProvider(t *testing.T) {
	methods, _ := resolverWith(t, `{"google":false}`, authmethod.Configured{Google: true})
	h := &OAuthStartHandler{
		Providers:   map[string]oauth.Provider{"google": providerWithClientID("client-id")},
		StateSecret: []byte("s"),
		StateTTL:    5 * time.Minute,
		Methods:     methods,
	}
	rec := httptest.NewRecorder()
	h.Start(rec, startRequest("google"))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a disabled provider, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOAuthStartRefusesProviderTheDeploymentCannotRun(t *testing.T) {
	// Switched on in the admin panel, but no credentials: the intersection in
	// authmethod.Availability is what has to stop this.
	methods, _ := resolverWith(t, `{"github":true}`, authmethod.Configured{GitHub: false})
	h := &OAuthStartHandler{
		Providers:   map[string]oauth.Provider{"github": providerWithClientID("")},
		StateSecret: []byte("s"),
		StateTTL:    5 * time.Minute,
		Methods:     methods,
	}
	rec := httptest.NewRecorder()
	h.Start(rec, startRequest("github"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestOAuthStartAllowsConfiguredAndEnabledProvider(t *testing.T) {
	methods, _ := resolverWith(t, "", authmethod.Configured{Google: true})
	h := &OAuthStartHandler{
		Providers:   map[string]oauth.Provider{"google": providerWithClientID("client-id")},
		StateSecret: []byte("s"),
		StateTTL:    5 * time.Minute,
		Methods:     methods,
	}
	rec := httptest.NewRecorder()
	h.Start(rec, startRequest("google"))

	if rec.Code/100 != 3 {
		t.Fatalf("expected a redirect, got %d: %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "provider.example") {
		t.Errorf("expected the provider authorize URL, got %q", loc)
	}
}

func TestAuthMethodsPublicReportsAvailabilityNotSwitches(t *testing.T) {
	// Google is switched on but has no credentials: the public answer must be
	// false so the login page does not render a broken button.
	methods, _ := resolverWith(t, `{"google":true,"password_registration":true}`, authmethod.Configured{Google: false, GitHub: true})
	h := &AuthMethodsHandler{Methods: methods}

	rec := httptest.NewRecorder()
	h.Public(rec, httptest.NewRequest("GET", "/api/auth/methods", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Data authmethod.Availability `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Google {
		t.Error("google must be reported unavailable without credentials")
	}
	if !body.Data.GitHub {
		t.Error("github is configured and on, so it should be available")
	}
	if !body.Data.PasswordRegistration {
		t.Error("password registration is on")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("the answer must not be cached, got %q", got)
	}
}

func TestAdminSetIsAPartialUpdate(t *testing.T) {
	// Disabling one provider must not disturb the other switches. Decoding the
	// body straight into Settings would have turned every omitted field off.
	methods, _ := resolverWith(t, `{"password_registration":true,"google":true,"github":true}`,
		authmethod.Configured{Google: true, GitHub: true})
	h := &AuthMethodsHandler{Methods: methods}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/admin/auth-methods", strings.NewReader(`{"google":false}`))
	h.AdminSet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}

	got := methods.Settings(context.Background())
	if got.Google {
		t.Error("google should now be off")
	}
	if !got.PasswordRegistration || !got.GitHub {
		t.Errorf("omitted fields must keep their value, got %+v", got)
	}
}

func TestAdminGetReportsConfiguredState(t *testing.T) {
	methods, _ := resolverWith(t, "", authmethod.Configured{Google: true, GitHub: false})
	h := &AuthMethodsHandler{Methods: methods}

	rec := httptest.NewRecorder()
	h.AdminGet(rec, httptest.NewRequest("GET", "/api/admin/auth-methods", nil))

	var body struct {
		Data authmethod.AdminView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Data.Configured.Google || body.Data.Configured.GitHub {
		t.Errorf("configured state wrong: %+v", body.Data.Configured)
	}
	if body.Data.Customized {
		t.Error("no stored row means not customized")
	}
}
