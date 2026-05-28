package handler

import (
	"net/http"
	"strconv"

	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type SearchHandler struct {
	searchStore *postgres.SearchStore
}

func NewSearchHandler(ss *postgres.SearchStore) *SearchHandler {
	return &SearchHandler{searchStore: ss}
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if len(query) < 2 {
		http.Error(w, "query parameter 'q' is required and must be at least 2 characters", http.StatusBadRequest)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	result, err := h.searchStore.SearchAll(r.Context(), query, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, result)
}
