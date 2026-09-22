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
	// `visible` is the column the rest of the store filters on (see
	// idx_problems_visible and ProblemStore.List). An earlier version of this
	// query used `is_public`, which does not exist -- it errored, and the
	// caller's error handling turned that into an empty sitemap.
	rows, err := s.db.QueryContext(ctx,
		`SELECT slug, COALESCE(updated_at, created_at)
		   FROM problems
		  WHERE visible = true
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
	// contests has created_at but no updated_at, so the timestamp comes from
	// created_at alone. (The problems table does have updated_at.)
	rows, err := s.db.QueryContext(ctx,
		`SELECT slug, created_at
		   FROM contests
		  WHERE visible = true
		  ORDER BY created_at DESC
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

// PublicUsers returns user profiles worth crawling.
//
// Ported from dmoj's judge/sitemap.py UserSitemap. dmoj indexes every user; here
// the rule is narrower because AIOJ's users table has `role` and `is_bot`, so
// bot accounts and privileged staff accounts are excluded -- ranking pages for
// admin accounts invites enumeration of staff, and bot profiles are not
// meaningful search results.
func (s *SitemapStore) PublicUsers(ctx context.Context, origin string) ([]SitemapEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT username, created_at
		   FROM users
		  WHERE COALESCE(is_bot, false) = false
		    AND COALESCE(role, '') NOT IN ('admin', 'staff')
		  ORDER BY created_at DESC
		  LIMIT 50000`)
	if err != nil {
		return nil, fmt.Errorf("sitemap users: %w", err)
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var username string
		var createdAt time.Time
		if err := rows.Scan(&username, &createdAt); err != nil {
			return nil, fmt.Errorf("scan sitemap user: %w", err)
		}
		out = append(out, SitemapEntry{
			Location:   origin + "/users/" + username,
			LastMod:    createdAt,
			ChangeFreq: "monthly",
			Priority:   "0.3",
		})
	}
	return out, rows.Err()
}
