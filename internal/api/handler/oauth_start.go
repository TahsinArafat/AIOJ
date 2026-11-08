package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/authmethod"
	"github.com/tahsinarafat/aioj/internal/oauth"
)

// OAuthStartHandler redirects the browser to the provider authorize URL
// with a short-lived HMAC state token (CSRF protection).
type OAuthStartHandler struct {
	Providers   map[string]oauth.Provider
	StateSecret []byte
	StateTTL    time.Duration
	// Methods gates the providers an admin has switched off or that the
	// deployment has no credentials for. Nil means "no gate", which keeps the
	// handler usable in tests that build it as a literal.
	Methods *authmethod.Resolver
}

func (h *OAuthStartHandler) Start(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "provider")
	p, ok := h.Providers[name]
	if !ok {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}
	// An unconfigured provider has no client id, so AuthCodeURL would emit a
	// redirect that the provider answers with an error page. The login page
	// hides those buttons (see GET /api/auth/methods); this is the backstop for
	// a stale tab, a bookmark, or a hand-written link.
	if p.Config().ClientID == "" {
		http.Error(w, "provider is not configured on this instance", http.StatusNotFound)
		return
	}
	// Provider names are the same strings as the authmethod keys, so a provider
	// added to main.go without a matching key is refused here rather than
	// silently bypassing the admin's switch.
	if h.Methods != nil && !h.Methods.Available(r.Context(), name) {
		http.Error(w, "provider is disabled on this instance", http.StatusForbidden)
		return
	}
	raw, sig, err := oauth.IssueStateTokenWith(h.StateSecret, h.StateTTL)
	if err != nil {
		http.Error(w, "state issue: "+err.Error(), http.StatusInternalServerError)
		return
	}
	state := raw + "." + oauth.EncodeSig(sig)
	url := p.Config().AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}
