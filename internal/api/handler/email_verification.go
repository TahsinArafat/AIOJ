package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/store"
)

// EmailVerificationHandler completes email verification from a signed link.
type EmailVerificationHandler struct {
	Users  store.UserStore
	Tokens store.EmailVerificationTokenStore
}

func (h *EmailVerificationHandler) Verify(w http.ResponseWriter, r *http.Request) {
	raw := chi.URLParam(r, "token")
	if raw == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}
	sum := sha256.Sum256([]byte(raw))
	tok, err := h.Tokens.GetByHash(r.Context(), hex.EncodeToString(sum[:]))
	if err != nil || tok == nil {
		http.Error(w, "invalid token", http.StatusBadRequest)
		return
	}
	if tok.Used {
		http.Error(w, "token already used", http.StatusBadRequest)
		return
	}
	if time.Now().After(tok.ExpiresAt) {
		http.Error(w, "token expired", http.StatusBadRequest)
		return
	}
	if err := h.Users.MarkEmailVerified(r.Context(), tok.UserID); err != nil {
		http.Error(w, "failed to mark verified", http.StatusInternalServerError)
		return
	}
	_ = h.Tokens.MarkUsed(r.Context(), tok.ID)
	respondJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}
