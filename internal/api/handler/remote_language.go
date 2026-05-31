package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type RemoteLanguageHandler struct {
	store store.RemoteLanguageStore
}

func NewRemoteLanguageHandler(s store.RemoteLanguageStore) *RemoteLanguageHandler {
	return &RemoteLanguageHandler{store: s}
}

func (h *RemoteLanguageHandler) ListByPlatform(w http.ResponseWriter, r *http.Request) {
	platform := chi.URLParam(r, "platform")
	langs, err := h.store.ListByPlatform(r.Context(), platform)
	if err != nil {
		http.Error(w, "failed to list remote languages", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": langs})
}

func (h *RemoteLanguageHandler) Create(w http.ResponseWriter, r *http.Request) {
	var rl model.RemoteLanguage
	if err := json.NewDecoder(r.Body).Decode(&rl); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if rl.Platform == "" || rl.LocalID == "" || rl.RemoteID == "" {
		http.Error(w, "platform, local_id, and remote_id are required", http.StatusBadRequest)
		return
	}
	if err := h.store.Create(r.Context(), &rl); err != nil {
		http.Error(w, "failed to create remote language: "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusCreated, rl)
}

func (h *RemoteLanguageHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "failed to get remote language", http.StatusInternalServerError)
		return
	}
	if existing == nil {
		http.Error(w, "remote language not found", http.StatusNotFound)
		return
	}
	var req struct {
		LocalID             *string `json:"local_id"`
		RemoteID            *string `json:"remote_id"`
		DisplayName         *string `json:"display_name"`
		Enabled             *bool   `json:"enabled"`
		SortOrder           *int    `json:"sort_order"`
		InlineCommentPrefix *string `json:"inline_comment_prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.LocalID != nil {
		existing.LocalID = *req.LocalID
	}
	if req.RemoteID != nil {
		existing.RemoteID = *req.RemoteID
	}
	if req.DisplayName != nil {
		existing.DisplayName = *req.DisplayName
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		existing.SortOrder = *req.SortOrder
	}
	if req.InlineCommentPrefix != nil {
		existing.InlineCommentPrefix = *req.InlineCommentPrefix
	}
	if err := h.store.Update(r.Context(), id, existing); err != nil {
		http.Error(w, "failed to update remote language", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

func (h *RemoteLanguageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), id); err != nil {
		http.Error(w, "failed to delete remote language", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
