package postgres

import (
	"context"
	"fmt"
)

// ContestFeedEntry is one contest rendered for a feed.
type ContestFeedEntry struct {
	Title   string
	Slug    string
	Summary string
	Created any
}

// RecentContests returns recent visible contests for the Atom feed.
//
// Ported from dmoj's judge/feed.py contest feed. AIOJ has no contest feed, so
// this follows the pattern of the problems feed. contests has both created_at
// and start_time; start_time is the event date and is the more useful ordering
// for a contest feed, but created_at is used so the feed stays "newest added
// first" like the others and does not reorder as contests are edited.
func (s *FeedStore) RecentContests(ctx context.Context, limit int) ([]ContestFeedEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT title, slug, COALESCE(description, ''), created_at
		   FROM contests
		  WHERE visible = true
		  ORDER BY created_at DESC, id DESC
		  LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("feed recent contests: %w", err)
	}
	defer rows.Close()

	out := []ContestFeedEntry{}
	for rows.Next() {
		var e ContestFeedEntry
		if err := rows.Scan(&e.Title, &e.Slug, &e.Summary, &e.Created); err != nil {
			return nil, fmt.Errorf("scan feed contest: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
