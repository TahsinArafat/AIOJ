package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type AdminBotAccountHandler struct {
	store store.BotAccountStore
}

func NewAdminBotAccountHandler(s store.BotAccountStore) *AdminBotAccountHandler {
	return &AdminBotAccountHandler{store: s}
}

func (h *AdminBotAccountHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	accounts, total, err := h.store.List(r.Context(), offset, limit)
	if err != nil {
		http.Error(w, "failed to list bot accounts", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": accounts, "total": total})
}

func (h *AdminBotAccountHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ba, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to get bot account", http.StatusInternalServerError)
		return
	}
	if ba == nil {
		http.Error(w, "bot account not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, ba)
}

func (h *AdminBotAccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID       string            `json:"user_id"`
		Platform     string            `json:"platform"`
		PlatformUser string            `json:"platform_user"`
		PlatformPass string            `json:"platform_pass"`
		APIKey       string            `json:"api_key"`
		APISecret    string            `json:"api_secret"`
		SessionData  map[string]string `json:"session_data"`
		RateLimitRPS float32           `json:"rate_limit_rps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Platform == "" || req.UserID == "" {
		http.Error(w, "user_id and platform are required", http.StatusBadRequest)
		return
	}
	if req.RateLimitRPS <= 0 {
		req.RateLimitRPS = 1.0
	}
	ba := &model.BotAccount{
		UserID:       req.UserID,
		Platform:     req.Platform,
		PlatformUser: req.PlatformUser,
		PlatformPass: req.PlatformPass,
		APIKey:       req.APIKey,
		APISecret:    req.APISecret,
		SessionData:  req.SessionData,
		Status:       "active",
		RateLimitRPS: req.RateLimitRPS,
	}
	if err := h.store.Create(r.Context(), ba); err != nil {
		http.Error(w, "failed to create bot account: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, ba)
}

func (h *AdminBotAccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to get bot account", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "bot account not found", http.StatusNotFound)
		return
	}
	var req struct {
		PlatformUser *string  `json:"platform_user"`
		PlatformPass *string  `json:"platform_pass"`
		Status       *string  `json:"status"`
		RateLimitRPS *float32 `json:"rate_limit_rps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.PlatformUser != nil {
		existing.PlatformUser = *req.PlatformUser
	}
	if req.PlatformPass != nil {
		existing.PlatformPass = *req.PlatformPass
	}
	if req.Status != nil {
		validStatuses := map[string]bool{"active": true, "expired": true, "error": true, "banned": true}
		if !validStatuses[*req.Status] {
			http.Error(w, "invalid status: must be active, expired, error, or banned", http.StatusBadRequest)
			return
		}
		existing.Status = *req.Status
	}
	if req.RateLimitRPS != nil {
		existing.RateLimitRPS = *req.RateLimitRPS
	}
	if err := h.store.Update(r.Context(), id, existing); err != nil {
		http.Error(w, "failed to update bot account", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

func (h *AdminBotAccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), id); err != nil {
		http.Error(w, "failed to delete bot account", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
