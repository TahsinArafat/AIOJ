package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store"
)

// store.SubmissionStats is already the narrow seam here: the three aggregate
// queries are what a stats page is, and all three are used.
type StatsHandler struct {
	subStore     store.SubmissionStats
	problemStore problemReadAccess
}

func NewStatsHandler(ss store.SubmissionStats, ps store.ProblemStore) *StatsHandler {
	return &StatsHandler{subStore: ss, problemStore: ps}
}

func (h *StatsHandler) GetProblemStats(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "problemId")

	// Per-problem solve statistics are a draft's activity, and answering them is
	// also an existence oracle for unpublished problems if left unguarded.
	p, err := h.problemStore.GetByID(r.Context(), problemID)
	if err != nil || p == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !canReadProblem(r.Context(), h.problemStore, p, middleware.GetUserClaims(r)) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	stats, err := h.subStore.GetProblemStats(r.Context(), problemID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

func (h *StatsHandler) GetPlatformStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.subStore.GetPlatformStats(r.Context())
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
