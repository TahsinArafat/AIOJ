package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/oauth"
)

// OAuthStartHandler redirects the browser to the provider authorize URL
// with a short-lived HMAC state token (CSRF protection).
type OAuthStartHandler struct {
	Providers   map[string]oauth.Provider
	StateSecret []byte
	StateTTL    time.Duration
}

func (h *OAuthStartHandler) Start(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "provider")
	p, ok := h.Providers[name]
	if !ok {
		http.Error(w, "unknown provider", http.StatusBadRequest)
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
