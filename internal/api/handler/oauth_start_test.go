package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/oauth"
	"golang.org/x/oauth2"
)

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
