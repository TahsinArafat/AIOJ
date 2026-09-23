package handler

import (
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

// ActivityHandler serves GET /api/activity — unified recent community feed.
type ActivityHandler struct {
	Subs     *postgres.SubmissionStore
	Ratings  *postgres.RatingStore
	Comments *postgres.FeedStore
}

func NewActivityHandler(s *postgres.SubmissionStore, r *postgres.RatingStore, c *postgres.FeedStore) *ActivityHandler {
	return &ActivityHandler{Subs: s, Ratings: r, Comments: c}
}

// List returns the newest N items across AC, rating changes, and comments.
// Query: ?limit=30 (default 30, max 100).
func (h *ActivityHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	limit := 30
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	// Collect in parallel would be nicer; sequential is fine for MVP.
	var items []postgres.ActivityItem
	if h.Subs != nil {
		ac, err := h.Subs.RecentAC(ctx, limit)
		if err != nil {
			slog.Warn("activity ac", "error", err)
		}
		items = append(items, ac...)
	}
	if h.Ratings != nil {
		rt, err := h.Ratings.RecentRatings(ctx, limit)
		if err != nil {
			slog.Warn("activity ratings", "error", err)
		}
		items = append(items, rt...)
	}
	if h.Comments != nil {
		cm, err := h.Comments.RecentCommentsActivity(ctx, limit)
		if err != nil {
			slog.Warn("activity comments", "error", err)
		}
		items = append(items, cm...)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	if items == nil {
		items = []postgres.ActivityItem{}
	}

	w.Header().Set("Cache-Control", "public, max-age=30")
	respondJSON(w, http.StatusOK, map[string]any{"data": items})
}
