package handler

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/oauth"
	"golang.org/x/oauth2"
)

func newStartHandler() *OAuthStartHandler {
	return &OAuthStartHandler{
		Providers: map[string]oauth.Provider{
			"fake": &fakeOAuthProvider{cfg: &oauth2.Config{
				ClientID: "id", ClientSecret: "sec", RedirectURL: "http://localhost/cb",
				Scopes: []string{"user:email"},
				Endpoint: oauth2.Endpoint{
					AuthURL: "http://provider.example/authorize",
				},
			}},
		},
		StateSecret: []byte("s"),
		StateTTL:    5 * time.Minute,
	}
}

// stateNextFromRedirect pulls the signed post-login destination back out of the
// provider redirect, so a test can assert what the callback will receive.
func stateNextFromRedirect(t *testing.T, loc string) string {
	t.Helper()
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("missing state in " + loc)
	}
	idx := strings.LastIndex(state, ".")
	if idx < 0 {
		t.Fatal("malformed state " + state)
	}
	raw, sigStr := state[:idx], state[idx+1:]
	sig, err := oauth.DecodeSig(sigStr)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := oauth.ValidateStateTokenWith([]byte("s"), raw, sig, 5*time.Minute); err != nil || !ok {
		t.Fatalf("state not valid: ok=%v err=%v", ok, err)
	}
	return oauth.StateTokenNext(raw)
}

func TestOAuthStart_CarriesNextInState(t *testing.T) {
	h := newStartHandler()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "fake")
	req := httptest.NewRequest("GET", "/api/auth/oauth/fake/start?next="+url.QueryEscape("/problems?difficulty=hard"), nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Start(rec, req)

	if got := stateNextFromRedirect(t, rec.Header().Get("Location")); got != "/problems?difficulty=hard" {
		t.Errorf("next = %q, want the page the visitor started from", got)
	}
}

func TestOAuthStart_DropsUnsafeNext(t *testing.T) {
	h := newStartHandler()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "fake")
	req := httptest.NewRequest("GET", "/api/auth/oauth/fake/start?next="+url.QueryEscape("https://evil.example"), nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Start(rec, req)

	if got := stateNextFromRedirect(t, rec.Header().Get("Location")); got != "" {
		t.Errorf("next = %q, want an off-site destination dropped", got)
	}
}

func TestOAuthStart_RedirectsToProvider(t *testing.T) {
	h := &OAuthStartHandler{
		Providers: map[string]oauth.Provider{
			"fake": &fakeOAuthProvider{cfg: &oauth2.Config{
				ClientID: "id", ClientSecret: "sec", RedirectURL: "http://localhost/cb",
				Scopes: []string{"user:email"},
				Endpoint: oauth2.Endpoint{
					AuthURL: "http://provider.example/authorize",
				},
			}},
		},
		StateSecret: []byte("s"),
		StateTTL:    5 * time.Minute,
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "fake")
	req := httptest.NewRequest("GET", "/api/auth/oauth/fake/start", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Start(rec, req)

	if rec.Code/100 != 3 {
		t.Errorf("expected redirect (3xx), got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "http://provider.example/authorize") {
		t.Errorf("redirect URL wrong: %s", loc)
	}
	if !strings.Contains(loc, "state=") {
		t.Errorf("missing state: %s", loc)
	}
}
