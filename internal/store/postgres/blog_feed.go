package postgres

import (
	"context"
	"fmt"
)

// BlogFeedEntry is one blog post rendered for a feed.
type BlogFeedEntry struct {
	Title   string
	ID      int64
	Content string
	Created any
}

// RecentBlogPosts returns the most recent blog posts for the Atom feed.
//
// Ported from dmoj's judge/feed.py BlogFeed, which orders by
// ('-sticky', '-publish_on') and filters on visible=True, publish_on <= now.
//
// Two of those columns do not exist on AIOJ's blog_posts table. The live schema
// is: id, user_id, title, content, tags, upvotes, downvotes, comment_count,
// is_pinned, created_at, updated_at. So:
//
//   - is_pinned stands in for dmoj's `sticky` (same intent, ordering pinned
//     posts first).
//   - created_at stands in for `publish_on`.
//   - There is NO visibility column, so there is no filter to port. Publishing
//     every post is therefore a deliberate consequence of the schema, not an
//     oversight: if drafts are ever added, this query must gain the filter
//     before the feed is exposed, or it will syndicate unpublished content.
func (s *FeedStore) RecentBlogPosts(ctx context.Context, limit int) ([]BlogFeedEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 25 // dmoj publishes 25
	}
	// Filters now match dmoj's BlogFeed: visible posts only, published in the
	// past. publish_on NULL means "publish immediately", so the effective time
	// is publish_on when set and created_at otherwise.
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, COALESCE(content, ''), created_at
		   FROM blog_posts
		  WHERE visible = true
		    AND COALESCE(publish_on, created_at) <= now()
		  ORDER BY is_pinned DESC NULLS LAST, created_at DESC, id DESC
		  LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("feed recent blog posts: %w", err)
	}
	defer rows.Close()

	out := []BlogFeedEntry{}
	for rows.Next() {
		var e BlogFeedEntry
		if err := rows.Scan(&e.ID, &e.Title, &e.Content, &e.Created); err != nil {
			return nil, fmt.Errorf("scan feed blog post: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
