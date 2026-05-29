package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type AdminSystemSettingsHandler struct {
	store store.SystemSettingsStore
}

func NewAdminSystemSettingsHandler(s store.SystemSettingsStore) *AdminSystemSettingsHandler {
	return &AdminSystemSettingsHandler{store: s}
}

func (h *AdminSystemSettingsHandler) List(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list settings", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": settings})
}

func (h *AdminSystemSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	setting, err := h.store.Get(r.Context(), key)
	if err != nil {
		http.Error(w, "failed to get setting", http.StatusInternalServerError)
		return
	}
	if setting == nil {
		http.Error(w, "setting not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, setting)
}

func (h *AdminSystemSettingsHandler) Set(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	var req struct {
		Key         string          `json:"key"`
		Value       json.RawMessage `json:"value"`
		Description string          `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}
	if req.Value == nil {
		req.Value = json.RawMessage(`{}`)
	}
	setting := &model.SystemSetting{
		Key:         req.Key,
		Value:       req.Value,
		Description: req.Description,
	}
	if claims != nil {
		setting.UpdatedBy = &claims.UserID
	}
	if err := h.store.Set(r.Context(), setting); err != nil {
		http.Error(w, "failed to set setting", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, setting)
}

func (h *AdminSystemSettingsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if err := h.store.Delete(r.Context(), key); err != nil {
		http.Error(w, "failed to delete setting", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
