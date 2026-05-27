package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type AdminHandler struct {
	userStore   store.UserStore
	setterStore store.SetterStore
}

func NewAdminHandler(u store.UserStore, s store.SetterStore) *AdminHandler {
	return &AdminHandler{userStore: u, setterStore: s}
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	users, total, _ := h.userStore.ListUsers(r.Context(), offset, limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": users, "total": total})
}

func (h *AdminHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	validRoles := map[string]bool{"admin": true, "teacher": true, "user": true, "bot": true}
	if !validRoles[req.Role] {
		http.Error(w, "invalid role: must be admin, teacher, user, or bot", http.StatusBadRequest)
		return
	}
	if err := h.userStore.UpdateRole(r.Context(), userID, req.Role); err != nil {
		http.Error(w, "failed to update role", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) ListSetterApps(w http.ResponseWriter, r *http.Request) {
	apps, _ := h.setterStore.ListApplications(r.Context())
	if apps == nil {
		apps = []model.SetterApplication{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": apps})
}

func (h *AdminHandler) ReviewSetterApp(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Status != "approved" && req.Status != "rejected" {
		http.Error(w, "invalid status: must be approved or rejected", http.StatusBadRequest)
		return
	}
	if err := h.setterStore.UpdateApplicationStatus(r.Context(), userID, req.Status); err != nil {
		http.Error(w, "failed to update application", http.StatusInternalServerError)
		return
	}
	if req.Status == "approved" {
		if err := h.userStore.UpdateRole(r.Context(), userID, "teacher"); err != nil {
			http.Error(w, "failed to update user role", http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) ApplySetter(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.setterStore.CreateApplication(r.Context(), claims.UserID, req.Reason); err != nil {
		http.Error(w, "failed to submit application", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *AdminHandler) GetSetterStatus(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	app, err := h.setterStore.GetApplication(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "failed to get application status", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, app)
}
