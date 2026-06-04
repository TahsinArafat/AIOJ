package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
	"golang.org/x/crypto/bcrypt"
)

type UsersHandler struct {
	userStore  store.UserStore
	blogStore  *postgres.BlogStore
	subStore   store.SubmissionStore
	teamStore  store.TeamStore
	groupStore store.GroupStore
}

func NewUsersHandler(us store.UserStore, bs *postgres.BlogStore, ss store.SubmissionStore, ts store.TeamStore, gs store.GroupStore) *UsersHandler {
	return &UsersHandler{userStore: us, blogStore: bs, subStore: ss, teamStore: ts, groupStore: gs}
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

// GetProfile returns the full profile for the authenticated user.
func (h *UsersHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	profile, err := h.userStore.GetProfile(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if profile == nil {
		http.Error(w, "profile not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, profile)
}

// UpdateProfile updates the profile fields for the authenticated user.
func (h *UsersHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req model.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	profile, err := h.userStore.GetProfile(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if profile == nil {
		http.Error(w, "profile not found", http.StatusNotFound)
		return
	}

	profile.Bio = req.Bio
	profile.AvatarURL = req.AvatarURL
	profile.FirstName = req.FirstName
	profile.LastName = req.LastName
	profile.Country = req.Country
	profile.City = req.City
	profile.Organization = req.Organization
	profile.GithubURL = req.GithubURL
	if req.ShowEmail != nil {
		profile.ShowEmail = *req.ShowEmail
	}
	if req.ShowTags != nil {
		profile.ShowTags = *req.ShowTags
	}

	if err := h.userStore.UpdateProfile(r.Context(), claims.UserID, profile); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, profile)
}

// UpdatePassword verifies current password and updates to new one.
func (h *UsersHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req model.UpdatePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "current_password and new_password required", http.StatusBadRequest)
		return
	}
	if len(req.NewPassword) < 6 {
		http.Error(w, "new password too short (min 6)", http.StatusBadRequest)
		return
	}

	user, err := h.userStore.GetByID(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		http.Error(w, "current password is incorrect", http.StatusUnauthorized)
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.userStore.UpdatePassword(r.Context(), claims.UserID, string(newHash)); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "password updated"})
}

// GetUserPendingInvites returns pending team and group invites for the auth user.
func (h *UsersHandler) GetUserPendingInvites(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamInvites, err := h.teamStore.GetUserPendingInvites(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if teamInvites == nil {
		teamInvites = []model.PendingInvite{}
	}

	groupInvites, err := h.groupStore.GetUserPendingInvites(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if groupInvites == nil {
		groupInvites = []model.GroupPendingInvite{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"teams":  teamInvites,
		"groups": groupInvites,
	})
}
