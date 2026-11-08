package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

const oauthTestSecret = "state-secret-for-tests"

// oauthCallbackFixture wires the callback handler against in-memory stubs, so
// the whole "ticket then username" exchange can be driven without a database.
type oauthCallbackFixture struct {
	handler *OAuthCallbackHandler
	users   *stubUserForEV
	links   *stubOAuthLinkStore
	secret  []byte
}

func newOAuthCallbackFixture(t *testing.T) *oauthCallbackFixture {
	t.Helper()
	tokSrv := newTokenEndpoint(t)
	t.Cleanup(tokSrv.Close)

	users := &stubUserForEV{users: map[string]*model.User{}}
	links := &stubOAuthLinkStore{links: map[string]*model.OAuthLink{}}
	return &oauthCallbackFixture{
		users:  users,
		links:  links,
		secret: []byte(oauthTestSecret),
		handler: &OAuthCallbackHandler{
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
			JWT:         auth.NewJWTManager("test-secret", time.Minute, 24*time.Hour),
			Refresh:     &stubRefresh{},
			StateTTL:    5 * time.Minute,
			StateSecret: []byte(oauthTestSecret),
			RespondJSON: true,
		},
	}
}

// callback drives GET /api/auth/oauth/{provider}/callback with a valid state.
func (f *oauthCallbackFixture) callback(t *testing.T, accept string) *httptest.ResponseRecorder {
	t.Helper()
	raw, sig, err := oauth.IssueStateTokenWith(f.secret, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	state := raw + "." + oauth.EncodeSig(sig)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("provider", "fake")
	req := httptest.NewRequest("GET", "/api/auth/oauth/fake/callback?code=anything&state="+state, nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	f.handler.Callback(rec, req)
	return rec
}

func (f *oauthCallbackFixture) complete(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/auth/oauth/complete", strings.NewReader(body))
	rec := httptest.NewRecorder()
	f.handler.CompleteSignup(rec, req)
	return rec
}

func decodeSignupChallenge(t *testing.T, rec *httptest.ResponseRecorder) model.OAuthSignupChallenge {
	t.Helper()
	var ch model.OAuthSignupChallenge
	if err := json.Unmarshal(rec.Body.Bytes(), &ch); err != nil {
		t.Fatalf("decode challenge: %v (body %s)", err, rec.Body.String())
	}
	return ch
}

func TestOAuthCallback_NewUserAsksForUsername(t *testing.T) {
	f := newOAuthCallbackFixture(t)
	rec := f.callback(t, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	ch := decodeSignupChallenge(t, rec)
	if !ch.RequiresUsername || ch.SignupToken == "" {
		t.Fatalf("expected a username challenge, got %+v", ch)
	}
	if ch.SuggestedUsername != "fuser" {
		t.Errorf("suggested username %q, want the provider handle", ch.SuggestedUsername)
	}
	// The whole point: no row and no link until a username is chosen.
	if len(f.users.users) != 0 {
		t.Errorf("must not create a user before a username is chosen, got %d", len(f.users.users))
	}
	if len(f.links.links) != 0 {
		t.Errorf("must not link the provider before the account exists, got %d", len(f.links.links))
	}
}

func TestOAuthCallback_BrowserRedirectCarriesSignupTicket(t *testing.T) {
	f := newOAuthCallbackFixture(t)
	f.handler.RespondJSON = false
	f.handler.PublicURL = "https://aioj.example"
	rec := f.callback(t, "text/html,application/xhtml+xml")
	if rec.Code != http.StatusFound {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://aioj.example/oauth/complete#") {
		t.Fatalf("redirect %q must land on the SPA completion page", loc)
	}
	if !strings.Contains(loc, "signup_token=") {
		t.Errorf("redirect %q must carry the sign-up ticket", loc)
	}
	if strings.Contains(loc, "access_token=") {
		t.Error("a first-time sign-in must not carry tokens before a username is chosen")
	}
}

func TestOAuthCallback_CompleteSignupCreatesChosenUsername(t *testing.T) {
	f := newOAuthCallbackFixture(t)
	ch := decodeSignupChallenge(t, f.callback(t, ""))

	body, _ := json.Marshal(model.OAuthSignupRequest{SignupToken: ch.SignupToken, Username: "Custom_Handle"})
	rec := f.complete(t, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp model.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" || resp.User == nil {
		t.Fatalf("expected tokens+user, got %+v", resp)
	}
	if resp.User.Username != "custom_handle" {
		t.Errorf("username %q, want the chosen handle lowercased", resp.User.Username)
	}
	if len(f.users.users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(f.users.users))
	}
	if _, ok := f.links.links["fake:p1"]; !ok {
		t.Error("expected the provider identity to be linked to the new account")
	}
	if f.users.marked != resp.User.ID {
		t.Errorf("OAuth sign-up should mark the address verified, marked %q", f.users.marked)
	}
}

func TestOAuthCallback_CompleteSignupRejectsTakenUsername(t *testing.T) {
	f := newOAuthCallbackFixture(t)
	f.users.users["u2"] = &model.User{ID: "u2", Username: "taken"}
	ch := decodeSignupChallenge(t, f.callback(t, ""))

	body, _ := json.Marshal(model.OAuthSignupRequest{SignupToken: ch.SignupToken, Username: "taken"})
	rec := f.complete(t, string(body))
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body %s", rec.Code, rec.Body.String())
	}
	if len(f.users.users) != 1 {
		t.Errorf("must not create a second account, got %d", len(f.users.users))
	}
}

func TestOAuthCallback_CompleteSignupRejectsBadUsername(t *testing.T) {
	f := newOAuthCallbackFixture(t)
	ch := decodeSignupChallenge(t, f.callback(t, ""))

	for _, bad := range []string{"ab", "has space", "-leading", "trailing_", "ünicode"} {
		body, _ := json.Marshal(model.OAuthSignupRequest{SignupToken: ch.SignupToken, Username: bad})
		rec := f.complete(t, string(body))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: expected 400, got %d body %s", bad, rec.Code, rec.Body.String())
		}
	}
	if len(f.users.users) != 0 {
		t.Errorf("must not create a user from an invalid handle, got %d", len(f.users.users))
	}
}

func TestOAuthCallback_CompleteSignupRejectsForgedTicket(t *testing.T) {
	f := newOAuthCallbackFixture(t)
	// A ticket signed with another secret must never mint an account.
	forged, err := oauth.IssueSignupToken([]byte("attacker"), &oauth.PendingSignup{
		Provider: "fake", ProviderUserID: "p1", SuggestedName: "mallory",
	}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(model.OAuthSignupRequest{SignupToken: forged, Username: "mallory"})
	rec := f.complete(t, string(body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a forged ticket, got %d body %s", rec.Code, rec.Body.String())
	}
	if len(f.users.users) != 0 {
		t.Error("a forged ticket must not create a user")
	}
}

func TestOAuthCallback_CompleteSignupIsIdempotent(t *testing.T) {
	f := newOAuthCallbackFixture(t)
	ch := decodeSignupChallenge(t, f.callback(t, ""))
	body, _ := json.Marshal(model.OAuthSignupRequest{SignupToken: ch.SignupToken, Username: "alice"})
	if rec := f.complete(t, string(body)); rec.Code != http.StatusOK {
		t.Fatalf("first completion: %d %s", rec.Code, rec.Body.String())
	}
	// Double submit (or a reload): must sign the same account in, not 409.
	rec := f.complete(t, string(body))
	if rec.Code != http.StatusOK {
		t.Fatalf("replay: expected 200, got %d body %s", rec.Code, rec.Body.String())
	}
	if len(f.users.users) != 1 {
		t.Errorf("replay must not create a second account, got %d", len(f.users.users))
	}
}

func TestOAuthCallback_ExistingLinkSignsInWithoutUsernameStep(t *testing.T) {
	f := newOAuthCallbackFixture(t)
	existing := &model.User{ID: "u1", Username: "returning", Email: "f@x.com"}
	f.users.users["u1"] = existing
	_ = f.links.Create(context.Background(), &model.OAuthLink{
		ID: "l1", UserID: "u1", Provider: "fake", ProviderUserID: "p1",
	})

	rec := f.callback(t, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var resp model.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.AccessToken == "" || resp.User == nil || resp.User.ID != "u1" {
		t.Fatalf("expected a session for the linked account, got %+v", resp)
	}
}

func TestOAuthCallback_ExistingEmailLinksInsteadOfCreating(t *testing.T) {
	f := newOAuthCallbackFixture(t)
	existing := &model.User{ID: "u1", Username: "already_here", Email: "f@x.com"}
	f.users.users["u1"] = existing
	f.users.byEmail = map[string]*model.User{"f@x.com": existing}

	rec := f.callback(t, "")
	var resp model.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.User == nil || resp.User.ID != "u1" {
		t.Fatalf("expected the existing account, got %+v", resp)
	}
	if len(f.users.users) != 1 {
		t.Errorf("must not create a duplicate account, got %d", len(f.users.users))
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

func TestOAuthUsernameAvailable(t *testing.T) {
	f := newOAuthCallbackFixture(t)
	ch := decodeSignupChallenge(t, f.callback(t, ""))
	f.users.users["u2"] = &model.User{ID: "u2", Username: "taken"}

	cases := []struct {
		name    string
		token   string
		handle  string
		status  int
		wantOK  bool
		wantMsg string
	}{
		{"free", ch.SignupToken, "fresh", http.StatusOK, true, ""},
		{"taken", ch.SignupToken, "taken", http.StatusOK, false, "username taken"},
		{"invalid format", ch.SignupToken, "no spaces", http.StatusOK, false, "username"},
		{"no ticket", "", "fresh", http.StatusBadRequest, false, "ticket"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := url.Values{"signup_token": {tc.token}, "username": {tc.handle}}
			req := httptest.NewRequest("GET", "/api/auth/oauth/username-available?"+q.Encode(), nil)
			rec := httptest.NewRecorder()
			f.handler.UsernameAvailable(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status %d, want %d (%s)", rec.Code, tc.status, rec.Body.String())
			}
			var out model.OAuthUsernameCheck
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if out.Available != tc.wantOK {
				t.Errorf("available = %v, want %v (%+v)", out.Available, tc.wantOK, out)
			}
			if tc.wantMsg != "" && !strings.Contains(strings.ToLower(out.Error), strings.ToLower(tc.wantMsg)) {
				t.Errorf("error %q, want it to mention %q", out.Error, tc.wantMsg)
			}
		})
	}
}
