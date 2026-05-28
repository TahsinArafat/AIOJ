package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type LanguageLimitHandler struct {
	langLimitStore store.LanguageLimitStore
	problemStore   store.ProblemStore
}

func NewLanguageLimitHandler(langLimitStore store.LanguageLimitStore, problemStore store.ProblemStore) *LanguageLimitHandler {
	return &LanguageLimitHandler{langLimitStore: langLimitStore, problemStore: problemStore}
}

func (h *LanguageLimitHandler) List(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	p, err := h.problemStore.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	limits, err := h.langLimitStore.GetByProblem(r.Context(), p.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if limits == nil {
		limits = []*model.LanguageLimit{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": limits})
}

func (h *LanguageLimitHandler) Set(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	slug := chi.URLParam(r, "slug")
	p, err := h.problemStore.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && !h.problemStore.HasAccess(r.Context(), p.ID, claims.UserID, "owner", "co_author") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req struct {
		LanguageID    string `json:"language_id"`
		TimeLimitMs   *int   `json:"time_limit_ms,omitempty"`
		MemoryLimitKB *int   `json:"memory_limit_kb,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.LanguageID == "" {
		http.Error(w, "language_id required", http.StatusBadRequest)
		return
	}

	limit := &model.LanguageLimit{
		ProblemID:     p.ID,
		LanguageID:    req.LanguageID,
		TimeLimitMs:   req.TimeLimitMs,
		MemoryLimitKB: req.MemoryLimitKB,
	}
	if err := h.langLimitStore.Set(r.Context(), limit); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, limit)
}

func (h *LanguageLimitHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	slug := chi.URLParam(r, "slug")
	p, err := h.problemStore.GetBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if claims.Role != "admin" && !h.problemStore.HasAccess(r.Context(), p.ID, claims.UserID, "owner", "co_author") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	langID := chi.URLParam(r, "lang")
	if err := h.langLimitStore.Delete(r.Context(), p.ID, langID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
