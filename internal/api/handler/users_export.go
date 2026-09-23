package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

// UserDataAggregator pulls the GDPR export payload pieces from stores.
type UserDataAggregator interface {
	User(ctx context.Context, id string) (*model.User, error)
	Profile(ctx context.Context, id string) (*model.UserProfile, error)
	SubmissionCount(ctx context.Context, id string) (int, error)
}

// UsersExportHandler serves GET /api/users/me/export.
type UsersExportHandler struct {
	Data UserDataAggregator
}

// ExportMyData returns a JSON snapshot of the caller's account data.
func (h *UsersExportHandler) ExportMyData(w http.ResponseWriter, r *http.Request) {
	claims, ok := mustClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	u, err := h.Data.User(r.Context(), claims.UserID)
	if err != nil || u == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	p, _ := h.Data.Profile(r.Context(), claims.UserID)
	count, _ := h.Data.SubmissionCount(r.Context(), claims.UserID)

	respondJSON(w, http.StatusOK, map[string]any{
		"user":             u,
		"profile":          p,
		"submission_count": count,
		"exported_at":      time.Now().UTC(),
	})
}
