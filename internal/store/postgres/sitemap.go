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
	// A sitemap is submitted to search engines and is served to anonymous crawlers,
	// so it may only contain rounds that need no invitation. The previous query
	// filtered on visible alone and published password-protected contest URLs to
	// crawlers, from a method whose own comment claimed otherwise.
	rows, err := s.db.QueryContext(ctx,
		`SELECT slug, created_at
		   FROM contests
		  WHERE visible = true
		    AND password_hash = ''
		    AND NOT EXISTS (SELECT 1 FROM group_contests gc WHERE gc.contest_id = contests.id)
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

// PublicBlogPosts returns published blog posts worth crawling.
//
// The predicate is the same one ListPosts and the Atom feed use (migration
// 000058): visible, and publish_on arrived, where a NULL publish_on means
// "publish immediately". A scheduled or hidden post must not be advertised to a
// crawler even though it exists.
func (s *SitemapStore) PublicBlogPosts(ctx context.Context, origin string) ([]SitemapEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT bp.id, COALESCE(bp.updated_at, bp.created_at)
		   FROM blog_posts bp
		  WHERE COALESCE(bp.visible, true) = true
		    AND COALESCE(bp.publish_on, bp.created_at) <= now()
		  ORDER BY bp.created_at DESC
		  LIMIT 50000`)
	if err != nil {
		return nil, fmt.Errorf("sitemap blog posts: %w", err)
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var id string
		var lastMod time.Time
		if err := rows.Scan(&id, &lastMod); err != nil {
			return nil, fmt.Errorf("scan sitemap blog post: %w", err)
		}
		out = append(out, SitemapEntry{
			Location:   origin + "/blog/" + id,
			LastMod:    lastMod,
			ChangeFreq: "monthly",
			Priority:   "0.5",
		})
	}
	return out, rows.Err()
}

// PublicEditorials returns editorials whose round has ended.
//
// editorialVisible is the same SQL function EditorialStore.GetByID filters on, so
// the sitemap and the page agree: an editorial for a round still running is not
// published, and listing it would point a crawler at a 404.
func (s *SitemapStore) PublicEditorials(ctx context.Context, origin string) ([]SitemapEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, COALESCE(e.updated_at, e.created_at)
		   FROM editorials e
		  WHERE editorialVisible(e.contest_id)
		  ORDER BY e.created_at DESC
		  LIMIT 50000`)
	if err != nil {
		return nil, fmt.Errorf("sitemap editorials: %w", err)
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var id string
		var lastMod time.Time
		if err := rows.Scan(&id, &lastMod); err != nil {
			return nil, fmt.Errorf("scan sitemap editorial: %w", err)
		}
		out = append(out, SitemapEntry{
			Location:   origin + "/editorials/" + id,
			LastMod:    lastMod,
			ChangeFreq: "monthly",
			Priority:   "0.5",
		})
	}
	return out, rows.Err()
}

// PublicGroups returns public groups worth crawling.
//
// is_public is the flag GroupStore.GetByID returns but does not filter on; the
// card resolver applies the same rule, so a group is either in both the sitemap
// and the preview or in neither.
func (s *SitemapStore) PublicGroups(ctx context.Context, origin string) ([]SitemapEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, COALESCE(g.updated_at, g.created_at)
		   FROM groups g
		  WHERE g.is_public = true
		  ORDER BY g.created_at DESC
		  LIMIT 50000`)
	if err != nil {
		return nil, fmt.Errorf("sitemap groups: %w", err)
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var id string
		var lastMod time.Time
		if err := rows.Scan(&id, &lastMod); err != nil {
			return nil, fmt.Errorf("scan sitemap group: %w", err)
		}
		out = append(out, SitemapEntry{
			Location:   origin + "/groups/" + id,
			LastMod:    lastMod,
			ChangeFreq: "monthly",
			Priority:   "0.4",
		})
	}
	return out, rows.Err()
}

// PublicTeams returns public teams worth crawling. Same is_public rule as
// PublicGroups.
func (s *SitemapStore) PublicTeams(ctx context.Context, origin string) ([]SitemapEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, COALESCE(t.updated_at, t.created_at)
		   FROM teams t
		  WHERE t.is_public = true
		  ORDER BY t.created_at DESC
		  LIMIT 50000`)
	if err != nil {
		return nil, fmt.Errorf("sitemap teams: %w", err)
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var id string
		var lastMod time.Time
		if err := rows.Scan(&id, &lastMod); err != nil {
			return nil, fmt.Errorf("scan sitemap team: %w", err)
		}
		out = append(out, SitemapEntry{
			Location:   origin + "/teams/" + id,
			LastMod:    lastMod,
			ChangeFreq: "monthly",
			Priority:   "0.4",
		})
	}
	return out, rows.Err()
}

// PublicOrganizations returns organizations worth crawling.
//
// Organizations have no visibility column and OrganizationStore.List returns them
// all, so there is nothing to filter on.
func (s *SitemapStore) PublicOrganizations(ctx context.Context, origin string) ([]SitemapEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT o.id, COALESCE(o.updated_at, o.created_at)
		   FROM organizations o
		  ORDER BY o.created_at DESC
		  LIMIT 50000`)
	if err != nil {
		return nil, fmt.Errorf("sitemap organizations: %w", err)
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var id string
		var lastMod time.Time
		if err := rows.Scan(&id, &lastMod); err != nil {
			return nil, fmt.Errorf("scan sitemap organization: %w", err)
		}
		out = append(out, SitemapEntry{
			Location:   origin + "/organizations/" + id,
			LastMod:    lastMod,
			ChangeFreq: "monthly",
			Priority:   "0.4",
		})
	}
	return out, rows.Err()
}

// PublicTrainingPlans returns the plans that are public.
//
// IsPublic is derived rather than stored: a plan owned by an organization is
// private to it, so the predicate is "no organization". TrainingPlanStore.GetByID
// computes the same thing, which is what keeps the sitemap and the card in step.
func (s *SitemapStore) PublicTrainingPlans(ctx context.Context, origin string) ([]SitemapEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tp.id, COALESCE(tp.updated_at, tp.created_at)
		   FROM training_plans tp
		  WHERE tp.organization_id IS NULL
		  ORDER BY tp.created_at DESC
		  LIMIT 50000`)
	if err != nil {
		return nil, fmt.Errorf("sitemap training plans: %w", err)
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var id string
		var lastMod time.Time
		if err := rows.Scan(&id, &lastMod); err != nil {
			return nil, fmt.Errorf("scan sitemap training plan: %w", err)
		}
		out = append(out, SitemapEntry{
			Location:   origin + "/training/" + id,
			LastMod:    lastMod,
			ChangeFreq: "monthly",
			Priority:   "0.4",
		})
	}
	return out, rows.Err()
}

// PublicGyms returns public gyms whose contest is published.
//
// Both halves matter: GymStore.List makes the same pair of conditions, because a
// gym pointing at an unpublished contest would otherwise advertise that contest's
// existence.
func (s *SitemapStore) PublicGyms(ctx context.Context, origin string) ([]SitemapEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, COALESCE(g.updated_at, g.created_at)
		   FROM gym_contests g
		   JOIN contests c ON g.contest_id = c.id
		  WHERE g.is_public = true
		    AND c.visible = true
		  ORDER BY g.created_at DESC
		  LIMIT 50000`)
	if err != nil {
		return nil, fmt.Errorf("sitemap gyms: %w", err)
	}
	defer rows.Close()

	out := []SitemapEntry{}
	for rows.Next() {
		var id string
		var lastMod time.Time
		if err := rows.Scan(&id, &lastMod); err != nil {
			return nil, fmt.Errorf("scan sitemap gym: %w", err)
		}
		out = append(out, SitemapEntry{
			Location:   origin + "/gym/" + id,
			LastMod:    lastMod,
			ChangeFreq: "monthly",
			Priority:   "0.4",
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
