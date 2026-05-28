package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type APIKeyHandler struct {
	store *postgres.APIKeyStore
}

func NewAPIKeyHandler(s *postgres.APIKeyStore) *APIKeyHandler {
	return &APIKeyHandler{store: s}
}

func (h *APIKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	secret, hash, err := postgres.GenerateAPIKey()
	if err != nil {
		http.Error(w, "key generation failed", http.StatusInternalServerError)
		return
	}

	rateLimit := req.RateLimit
	if rateLimit <= 0 {
		rateLimit = 100
	}

	k := &model.APIKey{
		UserID:      claims.UserID,
		KeyHash:     hash,
		Name:        req.Name,
		Description: req.Description,
		RateLimit:   rateLimit,
		IsActive:    true,
	}

	if err := h.store.Create(r.Context(), k); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, model.CreateAPIKeyResponse{
		APIKey: k,
		Secret: secret,
	})
}

func (h *APIKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	keys, err := h.store.GetByUser(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": keys})
}

func (h *APIKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), id); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
