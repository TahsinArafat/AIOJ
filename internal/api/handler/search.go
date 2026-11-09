package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

// searchStore is the one method this handler uses, so the redaction below can be
// tested without a database.
type searchStore interface {
	SearchAll(ctx context.Context, query string, limit int) (*postgres.SearchResult, error)
}

type SearchHandler struct {
	searchStore searchStore
}

func NewSearchHandler(ss searchStore) *SearchHandler {
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

	// Search is a public read too, and it ships difficulty and tags for every hit.
	// The dropdown only shows titles, but the JSON would answer "what difficulty is
	// problem C of the running round?".
	for i := range result.Problems {
		if result.Problems[i].InUnreleasedContest {
			redactProblemListItem(&result.Problems[i])
		}
	}

	respondJSON(w, http.StatusOK, result)
}
