package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// FeedStore gathers entries for the Atom feeds.
//
// Ported from dmoj's judge/feed.py, which exposes recently added public
// problems. Column sets here are taken from the live schema:
// problems(id, slug, title, description, visible, created_at) -- note there is
// no updated_at, which is what broke the first sitemap implementation.
type FeedStore struct {
	db *sql.DB
}

func NewFeedStore(db *sql.DB) *FeedStore {
	return &FeedStore{db: db}
}

// FeedEntry mirrors handler.FeedItem without importing the handler package.
type FeedEntry struct {
	Title   string
	Slug    string
	Summary string
	Created any
}

// RecentProblems returns the most recent visible problems, newest first.
func (s *FeedStore) RecentProblems(ctx context.Context, limit int) ([]FeedEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 25 // dmoj publishes 25
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT title, slug, COALESCE(description, ''), created_at
		   FROM problems
		  WHERE visible = true
		  ORDER BY created_at DESC, id DESC
		  LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("feed recent problems: %w", err)
	}
	defer rows.Close()

	out := []FeedEntry{}
	for rows.Next() {
		var e FeedEntry
		if err := rows.Scan(&e.Title, &e.Slug, &e.Summary, &e.Created); err != nil {
			return nil, fmt.Errorf("scan feed problem: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
