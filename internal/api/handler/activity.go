package handler

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

// The feed reads one method from each of three stores, so each is declared on
// the consumer side: this file depends on the three queries it runs rather than
// on the whole submission, rating and blog stores. postgres.ActivityItem stays
// as the shared row shape — it is what these queries return, not a dependency
// on the implementation.
type (
	recentACLister interface {
		RecentAC(ctx context.Context, limit int) ([]postgres.ActivityItem, error)
	}
	recentRatingLister interface {
		RecentRatings(ctx context.Context, limit int) ([]postgres.ActivityItem, error)
	}
	recentCommentLister interface {
		RecentCommentsActivity(ctx context.Context, limit int) ([]postgres.ActivityItem, error)
	}
)

// ActivityHandler serves GET /api/activity — unified recent community feed.
type ActivityHandler struct {
	Subs     recentACLister
	Ratings  recentRatingLister
	Comments recentCommentLister
}

func NewActivityHandler(s recentACLister, r recentRatingLister, c recentCommentLister) *ActivityHandler {
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
