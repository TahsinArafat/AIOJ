package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/tahsinarafat/aioj/internal/model"
)

// ratingRanker is the one query the rankings page runs. Declared on the
// consumer side so this handler does not depend on the whole user store for a
// single listing — it never reads, writes or bans a user.
type ratingRanker interface {
	ListUsersByRating(ctx context.Context, offset, limit int, country, organization string) ([]model.RankingEntry, int, error)
}

type RankingsHandler struct {
	userStore ratingRanker
}

func NewRankingsHandler(us ratingRanker) *RankingsHandler {
	return &RankingsHandler{userStore: us}
}

func (h *RankingsHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	country := r.URL.Query().Get("country")
	organization := r.URL.Query().Get("organization")
	users, total, err := h.userStore.ListUsersByRating(r.Context(), offset, limit, country, organization)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": users, "total": total})
}
