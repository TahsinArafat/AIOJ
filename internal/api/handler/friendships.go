package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store"
)

var errUserNotFound = errors.New("user not found")

// FriendshipHandler exposes follow/unfollow and list endpoints.
type FriendshipHandler struct {
	Friends store.FriendshipStore
	Users   store.UserStore
}

func NewFriendshipHandler(f store.FriendshipStore, u store.UserStore) *FriendshipHandler {
	return &FriendshipHandler{Friends: f, Users: u}
}

func (h *FriendshipHandler) resolveUserParam(r *http.Request) (string, error) {
	username := strings.TrimSpace(chi.URLParam(r, "username"))
	if username == "" {
		return "", errUserNotFound
	}
	u, err := h.Users.GetByUsername(r.Context(), username)
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", errUserNotFound
	}
	return u.ID, nil
}

// Follow: POST /api/users/{username}/follow
func (h *FriendshipHandler) Follow(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	followeeID, err := h.resolveUserParam(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	ok, err := h.Friends.Follow(r.Context(), claims.UserID, followeeID)
	if err != nil {
		http.Error(w, "follow failed", http.StatusBadRequest)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"following": true, "created": ok})
}

// Unfollow: DELETE /api/users/{username}/follow
func (h *FriendshipHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	followeeID, err := h.resolveUserParam(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err := h.Friends.Unfollow(r.Context(), claims.UserID, followeeID); err != nil {
		http.Error(w, "unfollow failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"following": false})
}

// Status: GET /api/users/{username}/follow
func (h *FriendshipHandler) Status(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	followeeID, err := h.resolveUserParam(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	following := false
	if claims != nil {
		following, _ = h.Friends.IsFollowing(r.Context(), claims.UserID, followeeID)
	}
	followingN, followersN, _ := h.Friends.Counts(r.Context(), followeeID)
	respondJSON(w, http.StatusOK, map[string]any{
		"following":      following,
		"following_count": followingN,
		"follower_count":  followersN,
	})
}

// ListFollowers: GET /api/users/{username}/followers
func (h *FriendshipHandler) ListFollowers(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveUserParam(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	items, err := h.Friends.ListFollowers(r.Context(), id)
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": items})
}

// ListFollowing: GET /api/users/{username}/following
func (h *FriendshipHandler) ListFollowing(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveUserParam(r)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	items, err := h.Friends.ListFollowing(r.Context(), id)
	if err != nil {
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": items})
}
