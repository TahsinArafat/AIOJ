package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type UsersHandler struct {
	userStore store.UserStore
	blogStore *postgres.BlogStore
	subStore  store.SubmissionStore
}

func NewUsersHandler(us store.UserStore, bs *postgres.BlogStore, ss store.SubmissionStore) *UsersHandler {
	return &UsersHandler{userStore: us, blogStore: bs, subStore: ss}
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

func (h *UsersHandler) GetUserSubmissions(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	profile, err := h.userStore.GetPublicProfile(r.Context(), username)
	if err != nil || profile == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	subs, total, err := h.subStore.ListPublicByUser(r.Context(), profile.ID, offset, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": subs, "total": total})
}

func (h *UsersHandler) GetUserBlogs(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	profile, err := h.userStore.GetPublicProfile(r.Context(), username)
	if err != nil || profile == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	posts, total, err := h.blogStore.ListByUser(r.Context(), profile.ID, offset, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": posts, "total": total})
}

func (h *UsersHandler) GetUserComments(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	profile, err := h.userStore.GetPublicProfile(r.Context(), username)
	if err != nil || profile == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	comments, total, err := h.blogStore.GetCommentsByUser(r.Context(), profile.ID, offset, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": comments, "total": total})
}
