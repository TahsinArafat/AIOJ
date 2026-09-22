package handler

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/oauth"
	"github.com/tahsinarafat/aioj/internal/store"
)

// OAuthCallbackHandler exchanges the provider code, loads or creates the
// local account, and issues JWT tokens.
type OAuthCallbackHandler struct {
	Users       store.UserStore
	Links       store.OAuthLinkStore
	Providers   map[string]oauth.Provider
	JWT         *auth.JWTManager
	Refresh     store.RefreshTokenStore
	StateTTL    time.Duration
	StateSecret []byte
	// PublicURL is the SPA origin used for browser redirects after login.
	PublicURL string
	// RespondJSON forces JSON instead of SPA fragment redirect (tests).
	RespondJSON bool
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
	if err := h.validateState(stateRaw); err != nil {
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
			h.issue(w, r, u)
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
			h.issue(w, r, existing)
			return
		}
	}

	username := h.uniqueUsername(r, info.Username)
	newUser := &model.User{
		ID: uuid.NewString(), Username: username, Email: info.Email, Role: "user",
	}
	if err := h.Users.Create(r.Context(), newUser); err != nil {
		http.Error(w, "create user: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if info.Email != "" {
		_ = h.Users.MarkEmailVerified(r.Context(), newUser.ID)
	}
	_ = h.Links.Create(r.Context(), &model.OAuthLink{
		ID: uuid.NewString(), UserID: newUser.ID,
		Provider: providerName, ProviderUserID: info.ProviderUserID,
	})
	h.issue(w, r, newUser)
}

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
		candidate = base + "_" + itoa(i)
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

func (h *OAuthCallbackHandler) issue(w http.ResponseWriter, r *http.Request, u *model.User) {
	access, _ := h.JWT.GenerateAccessToken(u.ID, u.Username, u.Role)
	raw, hashed := h.JWT.GenerateRefreshToken()
	_ = h.Refresh.Create(r.Context(), u.ID, hashed, time.Now().Add(h.JWT.RefreshTTL()))

	// Browser navigations get tokens in the URL fragment (never sent to server).
	// JSON clients (tests, API) get a plain AuthResponse body.
	wantJSON := h.RespondJSON || r.URL.Query().Get("format") == "json" ||
		(r.Header.Get("Accept") != "" && !containsHTML(r.Header.Get("Accept")))
	if !wantJSON {
		base := h.PublicURL
		if base == "" {
			base = "/"
		}
		uu, err := url.Parse(base)
		if err != nil {
			uu, _ = url.Parse("/")
		}
		uu.Path = "/oauth/complete"
		uu.Fragment = "access_token=" + url.QueryEscape(access) +
			"&refresh_token=" + url.QueryEscape(raw)
		http.Redirect(w, r, uu.String(), http.StatusFound)
		return
	}
	respondJSON(w, http.StatusOK, &model.AuthResponse{
		AccessToken: access, RefreshToken: raw, User: u,
	})
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

func (h *OAuthCallbackHandler) validateState(stateRaw string) error {
	idx := -1
	for i := len(stateRaw) - 1; i >= 0; i-- {
		if stateRaw[i] == '.' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return errors.New("state mismatch")
	}
	raw, sigStr := stateRaw[:idx], stateRaw[idx+1:]
	sig, err := oauth.DecodeSig(sigStr)
	if err != nil {
		return err
	}
	ok, err := oauth.ValidateStateTokenWith(h.StateSecret, raw, sig, h.StateTTL)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("state mismatch")
	}
	return nil
}
