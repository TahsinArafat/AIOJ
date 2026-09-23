package handler

import (
	"context"
	"net/http"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

// userLookupForDelete is the narrow read side used by account deletion.
type userLookupForDelete interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
}

// UserDeleter hard-deletes a user row; child rows follow FK cascade/SET NULL.
type UserDeleter interface {
	DeleteUserAndCascade(ctx context.Context, userID string) error
}

// UsersDeletionHandler serves DELETE /api/users/me.
type UsersDeletionHandler struct {
	Users   userLookupForDelete
	Deleter UserDeleter
}

// DeleteAccount requires password re-auth (when a local password exists) and
// confirm=true, then hard-deletes the caller.
func (h *UsersDeletionHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	claims, ok := mustClaims(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Password string `json:"password"`
		Confirm  bool   `json:"confirm"`
	}
	if err := bindJSON(r, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	u, err := h.Users.GetByID(r.Context(), claims.UserID)
	if err != nil || u == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	// Local-password accounts must re-auth; OAuth-only rows have an empty hash.
	if u.PasswordHash != "" && !auth.CheckPassword(req.Password, u.PasswordHash) {
		http.Error(w, "invalid password", http.StatusForbidden)
		return
	}
	if !req.Confirm {
		http.Error(w, "confirm=true required", http.StatusBadRequest)
		return
	}
	if err := h.Deleter.DeleteUserAndCascade(r.Context(), claims.UserID); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
