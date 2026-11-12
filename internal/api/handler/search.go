package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

// searchStore is the one method this handler uses, so the redaction below can be
// tested without a database.
type searchStore interface {
	SearchAll(ctx context.Context, query string, limit int, viewerID string) (*postgres.SearchResult, error)
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

	viewerID := ""
	if claims := middleware.GetUserClaims(r); claims != nil {
		viewerID = claims.UserID
	}
	result, err := h.searchStore.SearchAll(r.Context(), query, limit, viewerID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Problems still in play in a contest are already excluded from the archive
	// query, so nothing has to be stripped here.
	//
	// The archive is off limits to a generated contestant account, and search is
	// the second way to enumerate it, so its problem rows are dropped for that
	// role. Users and contests stay: the round's own pages use them.
	if isArchiveBlocked(middleware.GetUserClaims(r)) {
		result.Problems = []model.ProblemListItem{}
	}
	respondJSON(w, http.StatusOK, result)
}
