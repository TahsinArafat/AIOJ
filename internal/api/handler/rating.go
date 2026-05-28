package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type RatingHandler struct {
	ratingStore *postgres.RatingStore
}

func NewRatingHandler(rs *postgres.RatingStore) *RatingHandler {
	return &RatingHandler{ratingStore: rs}
}

func (h *RatingHandler) GetByUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		http.Error(w, "user ID required", http.StatusBadRequest)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	histories, err := h.ratingStore.GetByUser(r.Context(), userID, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": histories,
	})
}

func (h *RatingHandler) GetByContest(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "contestId")
	if contestID == "" {
		http.Error(w, "contest ID required", http.StatusBadRequest)
		return
	}

	histories, err := h.ratingStore.GetByContest(r.Context(), contestID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": histories,
	})
}
