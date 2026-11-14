package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/store"
)

// markEmailVerifier is the one user-store call the verification link makes.
// Narrower than store.UserCredentialStore, which also carries the password and
// ban operations this endpoint must not reach.
type markEmailVerifier interface {
	MarkEmailVerified(ctx context.Context, userID string) error
}

// EmailVerificationHandler completes email verification from a signed link.
type EmailVerificationHandler struct {
	Users  markEmailVerifier
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
