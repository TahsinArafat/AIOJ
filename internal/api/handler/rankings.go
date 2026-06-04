package handler

import (
	"net/http"
	"strconv"

	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type RankingsHandler struct {
	userStore *postgres.UserStore
}

func NewRankingsHandler(us *postgres.UserStore) *RankingsHandler {
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
