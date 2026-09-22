package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/oauth"
	"golang.org/x/oauth2"
)

type fakeOAuthProvider struct {
	cfg *oauth2.Config
	uid string
}

func (f *fakeOAuthProvider) Name() string           { return "fake" }
func (f *fakeOAuthProvider) Config() *oauth2.Config { return f.cfg }
func (f *fakeOAuthProvider) FetchUser(_ context.Context, _ *http.Client) (*oauth.UserInfo, error) {
	return &oauth.UserInfo{ProviderUserID: f.uid, Username: "fuser", Email: "f@x.com"}, nil
}

type stubOAuthLinkStore struct{ links map[string]*model.OAuthLink }

func (s *stubOAuthLinkStore) Create(_ context.Context, l *model.OAuthLink) error {
	if s.links == nil {
		s.links = map[string]*model.OAuthLink{}
	}
	s.links[l.Provider+":"+l.ProviderUserID] = l
	return nil
}
func (s *stubOAuthLinkStore) GetByProviderUser(_ context.Context, p, puid string) (*model.OAuthLink, error) {
	if s.links == nil {
		return nil, nil
	}
	return s.links[p+":"+puid], nil
}
func (s *stubOAuthLinkStore) ListByUser(context.Context, string) ([]model.OAuthLink, error) {
	return nil, nil
}
func (s *stubOAuthLinkStore) Delete(context.Context, string) error { return nil }

// newTokenEndpoint returns a mock OAuth2 token server for cfg.Exchange.
func newTokenEndpoint(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "at",
			"token_type":    "Bearer",
			"refresh_token": "rt",
			"expires_in":    3600,
		})
	}))
}

func TestOAuthCallback_CreatesUserAndLogsIn(t *testing.T) {
	tokSrv := newTokenEndpoint(t)
	defer tokSrv.Close()

	users := &stubUserForEV{users: map[string]*model.User{}}
	links := &stubOAuthLinkStore{links: map[string]*model.OAuthLink{}}
	secret := []byte("state-secret-for-tests")
	raw, sig, err := oauth.IssueStateTokenWith(secret, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	state := raw + "." + oauth.EncodeSig(sig)

	jwtMgr := auth.NewJWTManager("test-secret", time.Minute, 24*time.Hour)
	h := &OAuthCallbackHandler{
		Users: users,
		Links: links,
		Providers: map[string]oauth.Provider{
			"fake": &fakeOAuthProvider{
				uid: "p1",
				cfg: &oauth2.Config{
					ClientID:     "id",
					ClientSecret: "sec",
					RedirectURL:  "http://localhost/cb",
					Endpoint: oauth2.Endpoint{
						AuthURL:  "http://provider.example/authorize",
						TokenURL: tokSrv.URL,
					},
				},
			},
		},
		JWT:         jwtMgr,
		Refresh:     &stubRefresh{},
		StateTTL:    5 * time.Minute,
		StateSecret: secret,
		RespondJSON: true,
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "fake")
	req := httptest.NewRequest("GET", "/api/auth/oauth/fake/callback?code=anything&state="+state, nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Callback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if len(users.users) != 1 {
		t.Errorf("expected 1 new user, got %d", len(users.users))
	}
	if len(links.links) != 1 {
		t.Errorf("expected 1 oauth link, got %d", len(links.links))
	}
	var resp model.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.AccessToken == "" || resp.User == nil {
		t.Fatalf("expected tokens+user, got %+v", resp)
	}
}

func TestOAuthCallback_RejectsBadState(t *testing.T) {
	users := &stubUserForEV{users: map[string]*model.User{}}
	links := &stubOAuthLinkStore{links: map[string]*model.OAuthLink{}}
	h := &OAuthCallbackHandler{
		Users: users, Links: links,
		Providers:   map[string]oauth.Provider{"fake": &fakeOAuthProvider{uid: "p1"}},
		JWT:         auth.NewJWTManager("s", time.Minute, time.Hour),
		Refresh:     &stubRefresh{},
		StateTTL:    time.Minute,
		StateSecret: []byte("other-secret"),
		RespondJSON: true,
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "fake")
	req := httptest.NewRequest("GET", "/api/auth/oauth/fake/callback?code=x&state=dead.beef", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h.Callback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if len(users.users) != 0 {
		t.Error("should not create user on bad state")
	}
}
