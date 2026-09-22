package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SitemapEntry is one URL in the sitemap, with the timestamp a crawler uses to
// decide whether to re-fetch it.
type SitemapEntry struct {
	Location   string
	LastMod    time.Time
	ChangeFreq string
	Priority   string
}

// SitemapStore gathers the publicly crawlable URLs.
//
// Ported from dmoj's judge/sitemap.py, which exposes public problems and users.
// Only genuinely public, stable URLs are included: indexing a URL that 404s or
// requires auth is worse than omitting it, so login-only surfaces are skipped.
type SitemapStore struct {
	db *sql.DB
}

func NewSitemapStore(db *sql.DB) *SitemapStore {
	return &SitemapStore{db: db}
}

// PublicProblems returns problems visible to anonymous visitors.
func (s *SitemapStore) PublicProblems(ctx context.Context, origin string) ([]SitemapEntry, error) {
	// `is_public` mirrors the check the public /api/problems listing applies,
	// so the sitemap can't advertise a problem the API would hide.
	rows, err := s.db.QueryContext(ctx,
		`SELECT slug, COALESCE(updated_at, created_at)
		   FROM problems
		  WHERE is_public = true
		  ORDER BY updated_at DESC NULLS LAST
		  LIMIT 50000`)
	if err != nil {
		return nil, fmt.Errorf("sitemap problems: %w", err)
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var slug string
		var lastMod time.Time
		if err := rows.Scan(&slug, &lastMod); err != nil {
			return nil, fmt.Errorf("scan sitemap problem: %w", err)
		}
		out = append(out, SitemapEntry{
			Location:   origin + "/problems/" + slug,
			LastMod:    lastMod,
			ChangeFreq: "weekly",
			Priority:   "0.8",
		})
	}
	return out, rows.Err()
}

// PublicContests returns non-private contests.
func (s *SitemapStore) PublicContests(ctx context.Context, origin string) ([]SitemapEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT slug, COALESCE(updated_at, created_at)
		   FROM contests
		  WHERE is_public = true
		  ORDER BY updated_at DESC NULLS LAST
		  LIMIT 50000`)
	if err != nil {
		return nil, fmt.Errorf("sitemap contests: %w", err)
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var slug string
		var lastMod time.Time
		if err := rows.Scan(&slug, &lastMod); err != nil {
			return nil, fmt.Errorf("scan sitemap contest: %w", err)
		}
		out = append(out, SitemapEntry{
			Location:   origin + "/contests/" + slug,
			LastMod:    lastMod,
			ChangeFreq: "daily",
			Priority:   "0.7",
		})
	}
	return out, rows.Err()
}

// BlogPosts returns published posts.
func (s *SitemapStore) BlogPosts(ctx context.Context, origin string) ([]SitemapEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT slug, COALESCE(updated_at, created_at)
		   FROM blog_posts
		  WHERE is_published = true
		  ORDER BY created_at DESC
		  LIMIT 50000`)
	if err != nil {
		// A missing or differently-shaped blog table must not 500 the whole
		// sitemap -- the other sections are still worth serving.
		return []SitemapEntry{}, nil
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var slug string
		var lastMod time.Time
		if err := rows.Scan(&slug, &lastMod); err != nil {
			return []SitemapEntry{}, nil
		}
		out = append(out, SitemapEntry{
			Location:   origin + "/blog/" + slug,
			LastMod:    lastMod,
			ChangeFreq: "monthly",
			Priority:   "0.6",
		})
	}
	return out, nil
}
