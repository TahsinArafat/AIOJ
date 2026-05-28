package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store"
)

type StatsHandler struct {
	subStore store.SubmissionStore
}

func NewStatsHandler(ss store.SubmissionStore) *StatsHandler {
	return &StatsHandler{subStore: ss}
}

func (h *StatsHandler) GetProblemStats(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "problemId")
	stats, err := h.subStore.GetProblemStats(r.Context(), problemID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

func (h *StatsHandler) GetUserStats(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	stats, err := h.subStore.GetUserStats(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, stats)
}
