package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/vjudge"
)

type RemoteLanguageHandler struct {
	store     store.RemoteLanguageStore
	vjudgeSvc *vjudge.Service
}

func NewRemoteLanguageHandler(s store.RemoteLanguageStore, vjudgeSvc *vjudge.Service) *RemoteLanguageHandler {
	return &RemoteLanguageHandler{store: s, vjudgeSvc: vjudgeSvc}
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

func (h *RemoteLanguageHandler) AutoDetect(w http.ResponseWriter, r *http.Request) {
	platform := chi.URLParam(r, "platform")

	if h.vjudgeSvc == nil {
		http.Error(w, "vjudge service not available", http.StatusServiceUnavailable)
		return
	}

	remoteLangs, err := h.vjudgeSvc.FetchLanguages(r.Context(), platform)
	if err != nil {
		http.Error(w, "failed to fetch remote languages: "+err.Error(), http.StatusInternalServerError)
		return
	}

	existingLangs, err := h.store.ListByPlatform(r.Context(), platform)
	if err != nil {
		http.Error(w, "failed to list existing languages", http.StatusInternalServerError)
		return
	}

	type matchedItem struct {
		LocalID     string `json:"local_id"`
		RemoteID    string `json:"remote_id"`
		DisplayName string `json:"display_name"`
	}

	type unmatchedItem struct {
		RemoteID    string `json:"remote_id"`
		DisplayName string `json:"display_name"`
	}

	var matched []matchedItem
	var unmatched []unmatchedItem

	for _, remote := range remoteLangs {
		remoteName := strings.ToLower(remote.Name)
		found := false
		for _, existing := range existingLangs {
			// Check if any keyword from local language ID or display name appears in remote name
			keywords := []string{strings.ToLower(existing.LocalID), strings.ToLower(existing.DisplayName)}
			for _, kw := range keywords {
				if kw != "" && strings.Contains(remoteName, kw) {
					matched = append(matched, matchedItem{
						LocalID:     existing.LocalID,
						RemoteID:    remote.ID,
						DisplayName: remote.Name,
					})
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			unmatched = append(unmatched, unmatchedItem{
				RemoteID:    remote.ID,
				DisplayName: remote.Name,
			})
		}
	}

	if matched == nil {
		matched = []matchedItem{}
	}
	if unmatched == nil {
		unmatched = []unmatchedItem{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"matched":   matched,
		"unmatched": unmatched,
		"existing":  existingLangs,
	})
}

func (h *RemoteLanguageHandler) BulkUpsert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Platform  string                `json:"platform"`
		Languages []model.RemoteLanguage `json:"languages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Platform == "" {
		http.Error(w, "platform is required", http.StatusBadRequest)
		return
	}
	if len(req.Languages) == 0 {
		http.Error(w, "languages array is required", http.StatusBadRequest)
		return
	}

	if err := h.store.BulkUpsert(r.Context(), req.Platform, req.Languages); err != nil {
		http.Error(w, "failed to bulk upsert: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
