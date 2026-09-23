package postgres

import (
	"context"
	"fmt"
	"time"
)

// ActivityItem is a unified feed entry from submissions / ratings / comments.
type ActivityItem struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // ac | rating | comment
	Username  string    `json:"username"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary,omitempty"`
	Link      string    `json:"link,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// RecentAC returns recent accepted submissions on visible problems (public activity).
func (s *SubmissionStore) RecentAC(ctx context.Context, limit int) ([]ActivityItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id::text, COALESCE(u.username,'unknown'), COALESCE(p.title,''), s.created_at
		FROM submissions s
		JOIN users u ON u.id = s.user_id
		JOIN problems p ON p.id = s.problem_id
		WHERE s.status = 'ac' AND p.visible = TRUE
		ORDER BY s.created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent ac: %w", err)
	}
	defer rows.Close()
	out := []ActivityItem{}
	for rows.Next() {
		var a ActivityItem
		var subID string
		if err := rows.Scan(&subID, &a.Username, &a.Title, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Type = "ac"
		a.ID = "sub-" + subID
		a.Summary = "solved"
		a.Link = "/submissions/" + subID
		out = append(out, a)
	}
	return out, rows.Err()
}

// RecentRatings returns latest rating changes across all users.
func (s *RatingStore) RecentRatings(ctx context.Context, limit int) ([]ActivityItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT rh.id, COALESCE(u.username,'unknown'), rh.rating_change, rh.new_rating, rh.created_at
		FROM rating_history rh
		JOIN users u ON u.id = rh.user_id
		ORDER BY rh.created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent ratings: %w", err)
	}
	defer rows.Close()
	out := []ActivityItem{}
	for rows.Next() {
		var a ActivityItem
		var change, newR int
		if err := rows.Scan(&a.ID, &a.Username, &change, &newR, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.Type = "rating"
		sign := "+"
		if change < 0 {
			sign = ""
		}
		a.Title = fmt.Sprintf("rating %s%d → %d", sign, change, newR)
		a.Link = "/rankings"
		out = append(out, a)
	}
	return out, rows.Err()
}

// RecentCommentsActivity converts comment feed rows to ActivityItem.
func (s *FeedStore) RecentCommentsActivity(ctx context.Context, limit int) ([]ActivityItem, error) {
	entries, err := s.RecentComments(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]ActivityItem, 0, len(entries))
	for _, e := range entries {
		created, _ := e.Created.(time.Time)
		if created.IsZero() {
			if t, ok := e.Created.(time.Time); ok {
				created = t
			}
		}
		sum := e.Content
		if len(sum) > 120 {
			sum = sum[:117] + "..."
		}
		link := "/blog"
		switch e.ParentType {
		case "problem":
			link = "/problems"
		case "contest":
			link = "/contests"
		}
		out = append(out, ActivityItem{
			ID:        fmt.Sprintf("cmt-%d", e.ID),
			Type:      "comment",
			Username:  e.Username,
			Title:     "commented on " + e.ParentType,
			Summary:   sum,
			Link:      link,
			CreatedAt: created,
		})
	}
	return out, nil
}
