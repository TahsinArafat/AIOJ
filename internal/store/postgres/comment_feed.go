package postgres

import (
	"context"
	"fmt"
)

// CommentFeedEntry is one comment rendered for a feed.
type CommentFeedEntry struct {
	ID         int64
	Username   string
	ParentType string
	ParentID   int64
	Content    string
	Created    any
}

// RecentComments returns the most recent comments across all threads.
//
// Ported from dmoj's judge/feed.py CommentFeed. Comments carry parent_type and
// parent_id, so each entry links to the problem, contest or blog post it belongs
// to rather than a bare comment id.
//
// Ordering is by created_at DESC, id DESC. The id tiebreak matters: comments can
// share a created_at at second granularity, and without it the feed order is
// non-deterministic between requests, which makes readers re-report entries as
// updated.
func (s *FeedStore) RecentComments(ctx context.Context, limit int) ([]CommentFeedEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, COALESCE(u.username, 'unknown'), c.parent_type, c.parent_id,
		        COALESCE(c.content, ''), c.created_at
		   FROM comments c
		   LEFT JOIN users u ON u.id = c.user_id
		  ORDER BY c.created_at DESC, c.id DESC
		  LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("feed recent comments: %w", err)
	}
	defer rows.Close()

	out := []CommentFeedEntry{}
	for rows.Next() {
		var e CommentFeedEntry
		if err := rows.Scan(&e.ID, &e.Username, &e.ParentType, &e.ParentID, &e.Content, &e.Created); err != nil {
			return nil, fmt.Errorf("scan feed comment: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
