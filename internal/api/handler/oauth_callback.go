package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/oauth"
	"github.com/tahsinarafat/aioj/internal/store"
)

// oauthUserStore is the user surface a first-time OAuth sign-in needs: find the
// account a provider identity maps to, or mint one, and confirm the address the
// provider vouched for.
//
// Five operations, and no more — a callback handler has no business listing users
// or changing a role, both of which the field granted it.
type oauthUserStore interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	MarkEmailVerified(ctx context.Context, userID string) error
}

// OAuthCallbackHandler exchanges the provider code, loads or creates the
// local account, and issues JWT tokens.
type OAuthCallbackHandler struct {
	Users       oauthUserStore
	Links       store.OAuthLinkStore
	Providers   map[string]oauth.Provider
	JWT         *auth.JWTManager
	Refresh     store.RefreshTokenStore
	StateTTL    time.Duration
	StateSecret []byte
	// SignupTTL bounds how long a first-time sign-in may sit on the username
	// form before its ticket expires. Zero means signupTTLDefault.
	SignupTTL time.Duration
	// PublicURL is the SPA origin used for browser redirects after login.
	PublicURL string
	// RespondJSON forces JSON instead of SPA fragment redirect (tests).
	RespondJSON bool
}

// signupTTLDefault is long enough to read the form and pick a handle, short
// enough that a leaked ticket is not a durable way in.
const signupTTLDefault = 15 * time.Minute

func (h *OAuthCallbackHandler) signupTTL() time.Duration {
	if h.SignupTTL > 0 {
		return h.SignupTTL
	}
	return signupTTLDefault
}

func (h *OAuthCallbackHandler) Callback(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	prov, ok := h.Providers[providerName]
	if !ok {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	stateRaw := r.URL.Query().Get("state")
	if code == "" || stateRaw == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}
	next, err := h.validateState(stateRaw)
	if err != nil {
		http.Error(w, "invalid state: "+err.Error(), http.StatusBadRequest)
		return
	}

	cfg := prov.Config()
	tok, err := cfg.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "exchange failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	client := cfg.Client(r.Context(), tok)
	info, err := prov.FetchUser(r.Context(), client)
	if err != nil {
		http.Error(w, "fetch user failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	link, _ := h.Links.GetByProviderUser(r.Context(), providerName, info.ProviderUserID)
	if link != nil {
		u, _ := h.Users.GetByID(r.Context(), link.UserID)
		if u != nil {
			h.issue(w, r, u, next)
			return
		}
	}

	if info.Email != "" {
		existing, _ := h.Users.GetByEmail(r.Context(), info.Email)
		if existing != nil {
			_ = h.Links.Create(r.Context(), &model.OAuthLink{
				ID: uuid.NewString(), UserID: existing.ID,
				Provider: providerName, ProviderUserID: info.ProviderUserID,
			})
			h.issue(w, r, existing, next)
			return
		}
	}

	// No local account yet. The callback deliberately does not create one: a
	// first-time visitor gets to choose the handle everyone else will see
	// instead of being stuck with a mangled version of their Google/GitHub
	// display name. See CompleteSignup for the second half of the flow.
	h.beginSignup(w, r, &oauth.PendingSignup{
		Provider:       providerName,
		ProviderUserID: info.ProviderUserID,
		Email:          info.Email,
		SuggestedName:  h.uniqueUsername(r, oauth.SuggestUsername(info.Username)),
	}, next)
}

// beginSignup hands out a signed sign-up ticket: in the URL fragment for a
// browser navigation (so the identity never reaches a proxy log), as JSON for
// an API client.
func (h *OAuthCallbackHandler) beginSignup(w http.ResponseWriter, r *http.Request, p *oauth.PendingSignup, next string) {
	token, err := oauth.IssueSignupToken(h.StateSecret, p, h.signupTTL())
	if err != nil {
		http.Error(w, "signup token: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !h.wantsJSON(r) {
		vals := url.Values{}
		vals.Set("signup_token", token)
		vals.Set("username", p.SuggestedName)
		vals.Set("provider", p.Provider)
		if next != "" {
			vals.Set("next", next)
		}
		http.Redirect(w, r, h.completeURL().String()+"#"+vals.Encode(), http.StatusFound)
		return
	}
	respondJSON(w, http.StatusOK, &model.OAuthSignupChallenge{
		RequiresUsername:  true,
		SignupToken:       token,
		SuggestedUsername: p.SuggestedName,
		Provider:          p.Provider,
		Email:             p.Email,
	})
}

// CompleteSignup is the second half of a first-time OAuth sign-in: it turns a
// pending ticket plus a chosen username into a real account, links the
// provider identity, and signs the person in.
//
// The ticket — not a session — is the credential here, so this is safe to call
// before any account exists. Replaying it after the account was created signs
// the same person back in instead of failing on a taken handle.
func (h *OAuthCallbackHandler) CompleteSignup(w http.ResponseWriter, r *http.Request) {
	var req model.OAuthSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		signupError(w, http.StatusBadRequest, "invalid body")
		return
	}
	pending, err := oauth.ParseSignupToken(h.StateSecret, req.SignupToken)
	if err != nil {
		signupError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Already finished (double submit, or a reload of a form that went
	// through): sign the same account in rather than reject the handle it
	// now holds.
	if link, _ := h.Links.GetByProviderUser(r.Context(), pending.Provider, pending.ProviderUserID); link != nil {
		if u, _ := h.Users.GetByID(r.Context(), link.UserID); u != nil {
			h.issueJSON(w, r, u)
			return
		}
	}
	username, err := oauth.NormalizeUsername(req.Username)
	if err != nil {
		signupError(w, http.StatusBadRequest, err.Error())
		return
	}
	if existing, _ := h.Users.GetByUsername(r.Context(), username); existing != nil {
		signupError(w, http.StatusConflict, "username taken")
		return
	}
	// The provider vouched for this address, so an account created here is
	// verified. If a local account claimed the address in the meantime, link
	// to it instead of creating a duplicate.
	if pending.Email != "" {
		if existing, _ := h.Users.GetByEmail(r.Context(), pending.Email); existing != nil {
			_ = h.Links.Create(r.Context(), &model.OAuthLink{
				ID: uuid.NewString(), UserID: existing.ID,
				Provider: pending.Provider, ProviderUserID: pending.ProviderUserID,
			})
			h.issueJSON(w, r, existing)
			return
		}
	}
	newUser := &model.User{
		ID: uuid.NewString(), Username: username, Email: pending.Email, Role: model.RoleUser,
	}
	if err := h.Users.Create(r.Context(), newUser); err != nil {
		// Most likely lost a race with another sign-up for the same handle;
		// re-read it so the client gets "taken" and can retry rather than 500.
		if existing, _ := h.Users.GetByUsername(r.Context(), username); existing != nil {
			signupError(w, http.StatusConflict, "username taken")
			return
		}
		http.Error(w, "create user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if pending.Email != "" {
		_ = h.Users.MarkEmailVerified(r.Context(), newUser.ID)
	}
	_ = h.Links.Create(r.Context(), &model.OAuthLink{
		ID: uuid.NewString(), UserID: newUser.ID,
		Provider: pending.Provider, ProviderUserID: pending.ProviderUserID,
	})
	h.issueJSON(w, r, newUser)
}

// UsernameAvailable backs the live hint on the sign-up form. It requires a
// valid pending ticket so it cannot be used as a free username-enumeration
// endpoint by somebody without a half-finished sign-in.
func (h *OAuthCallbackHandler) UsernameAvailable(w http.ResponseWriter, r *http.Request) {
	if _, err := oauth.ParseSignupToken(h.StateSecret, r.URL.Query().Get("signup_token")); err != nil {
		respondJSON(w, http.StatusBadRequest, &model.OAuthUsernameCheck{Error: err.Error()})
		return
	}
	username, err := oauth.NormalizeUsername(r.URL.Query().Get("username"))
	if err != nil {
		respondJSON(w, http.StatusOK, &model.OAuthUsernameCheck{Error: err.Error()})
		return
	}
	if existing, _ := h.Users.GetByUsername(r.Context(), username); existing != nil {
		respondJSON(w, http.StatusOK, &model.OAuthUsernameCheck{Username: username, Error: "username taken"})
		return
	}
	respondJSON(w, http.StatusOK, &model.OAuthUsernameCheck{Available: true, Username: username})
}

func signupError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}

// uniqueUsername finds a free variant of base for the pre-filled suggestion.
// The handle rules cap a username at 32 characters, so a suffixed candidate is
// clipped to leave room for its suffix — otherwise the form would open with a
// suggestion it would then reject.
func (h *OAuthCallbackHandler) uniqueUsername(r *http.Request, base string) string {
	if base == "" {
		base = "user"
	}
	candidate := base
	for i := 2; ; i++ {
		existing, _ := h.Users.GetByUsername(r.Context(), candidate)
		if existing == nil {
			return candidate
		}
		suffix := "_" + itoa(i)
		trimmed := strings.TrimRight(base, "_.-")
		if room := oauth.UsernameMaxLen - len(suffix); len(trimmed) > room {
			trimmed = strings.TrimRight(trimmed[:room], "_.-")
		}
		candidate = trimmed + suffix
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// wantsJSON reports whether the caller can handle a JSON body instead of the
// SPA fragment redirect. Browser navigations advertise text/html.
func (h *OAuthCallbackHandler) wantsJSON(r *http.Request) bool {
	if h.RespondJSON || r.URL.Query().Get("format") == "json" {
		return true
	}
	accept := r.Header.Get("Accept")
	return accept != "" && !containsHTML(accept)
}

// completeURL is the SPA landing page for an OAuth callback.
func (h *OAuthCallbackHandler) completeURL() *url.URL {
	base := h.PublicURL
	if base == "" {
		base = "/"
	}
	uu, err := url.Parse(base)
	if err != nil {
		uu, _ = url.Parse("/")
	}
	uu.Path = "/oauth/complete"
	// The caller appends its own "#"+values; a fragment on the configured
	// origin would otherwise produce a second, stray "#".
	uu.Fragment = ""
	uu.RawFragment = ""
	return uu
}

func (h *OAuthCallbackHandler) issue(w http.ResponseWriter, r *http.Request, u *model.User, next string) {
	resp := h.mint(r, u)
	// Browser navigations get tokens in the URL fragment (never sent to server).
	// JSON clients (tests, API) get a plain AuthResponse body.
	if !h.wantsJSON(r) {
		vals := url.Values{}
		vals.Set("access_token", resp.AccessToken)
		vals.Set("refresh_token", resp.RefreshToken)
		if next != "" {
			vals.Set("next", next)
		}
		http.Redirect(w, r, h.completeURL().String()+"#"+vals.Encode(), http.StatusFound)
		return
	}
	respondJSON(w, http.StatusOK, resp)
}

// issueJSON always answers with a token body. CompleteSignup is called by
// fetch() (Accept: */*), which the redirect heuristic would otherwise send to
// the browser fragment flow.
func (h *OAuthCallbackHandler) issueJSON(w http.ResponseWriter, r *http.Request, u *model.User) {
	respondJSON(w, http.StatusOK, h.mint(r, u))
}

func (h *OAuthCallbackHandler) mint(r *http.Request, u *model.User) *model.AuthResponse {
	access, err := h.JWT.GenerateAccessTokenWithState(u.ID, u.Username, u.Role, u.MustChangePassword)
	if err != nil {
		slog.Error("oauth: generate access token failed", "user_id", u.ID, "error", err)
	}
	raw, hashed := h.JWT.GenerateRefreshToken()
	// A refresh token that was never persisted looks valid to the client but
	// fails the first time it is used, so the session dies at the access
	// token's expiry with no clue why. Log it like the password path does.
	if err := h.Refresh.Create(r.Context(), u.ID, hashed, time.Now().Add(h.JWT.RefreshTTL())); err != nil {
		slog.Error("oauth: persist refresh token failed", "user_id", u.ID, "error", err)
	}
	return &model.AuthResponse{AccessToken: access, RefreshToken: raw, User: u}
}

func containsHTML(accept string) bool {
	return len(accept) >= 9 && (containsFold(accept, "text/html") || containsFold(accept, "*/*"))
}

func containsFold(hay, needle string) bool {
	// small case-insensitive substring without strings import cost concerns
	if len(needle) == 0 || len(hay) < len(needle) {
		return len(needle) == 0
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		ok := true
		for j := 0; j < len(needle); j++ {
			a, b := hay[i+j], needle[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// validateState checks the provider state and returns the post-login
// destination it carries. The destination is signed, but NormalizeNext vets
// it again so a token minted by an older build can never become an open
// redirect.
func (h *OAuthCallbackHandler) validateState(stateRaw string) (string, error) {
	idx := -1
	for i := len(stateRaw) - 1; i >= 0; i-- {
		if stateRaw[i] == '.' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", errors.New("state mismatch")
	}
	raw, sigStr := stateRaw[:idx], stateRaw[idx+1:]
	sig, err := oauth.DecodeSig(sigStr)
	if err != nil {
		return "", err
	}
	ok, err := oauth.ValidateStateTokenWith(h.StateSecret, raw, sig, h.StateTTL)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("state mismatch")
	}
	return oauth.NormalizeNext(oauth.StateTokenNext(raw)), nil
}
