package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/store"
)

type UsersHandler struct {
	userStore store.UserStore
}

func NewUsersHandler(us store.UserStore) *UsersHandler {
	return &UsersHandler{userStore: us}
}

func (h *UsersHandler) GetByUsername(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	profile, err := h.userStore.GetPublicProfile(r.Context(), username)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if profile == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, profile)
}
