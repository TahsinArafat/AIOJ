package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type RatingHandler struct {
	ratingStore  *postgres.RatingStore
	contestStore *postgres.ContestStore
}

func NewRatingHandler(rs *postgres.RatingStore, cs *postgres.ContestStore) *RatingHandler {
	return &RatingHandler{ratingStore: rs, contestStore: cs}
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

// GetByContest serves rating deltas for a contest. The URL carries whatever
// identifier the user is looking at (display_id like "12" or a slug), but
// rating_history.contest_id is a UUID — feeding a display_id straight to the
// query made Postgres raise "invalid input syntax for type uuid" and the
// scoreboard got a 500 on every load. Resolve through ContestStore first.
func (h *RatingHandler) GetByContest(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "contestId")
	if contestID == "" {
		http.Error(w, "contest ID required", http.StatusBadRequest)
		return
	}

	resolved := contestID
	if h.contestStore != nil {
		contest, err := h.contestStore.GetByID(r.Context(), contestID)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if contest == nil {
			http.Error(w, "contest not found", http.StatusNotFound)
			return
		}
		resolved = contest.ID
	}

	histories, err := h.ratingStore.GetByContest(r.Context(), resolved)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": histories,
	})
}
