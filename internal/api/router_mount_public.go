package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tahsinarafat/aioj/internal/api/handler"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"net/http"
	"os"
)

func (b *routerBuilder) mountPublic(r chi.Router) {
	searchH := b.Search
	featuresH := b.Features
	// Metrics expose endpoint inventory, usernames in labels and traffic shape.
	// Prometheus scrapes with METRICS_TOKEN; an admin access token also works.
	r.With(middleware.MetricsAuth(b.MetricsToken, b.jwt)).Get("/metrics", promhttp.Handler().ServeHTTP)

	if b.Health != nil {
		r.Get("/api/health", b.Health.Health)
	} else {
		r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`{"status":"ok"}`)) })
	}

	if b.Activity != nil {
		r.Get("/api/activity", b.Activity.List)
	}

	// Feature gate decisions for the caller. Optional auth so a signed-in user
	// gets their role/rating verdicts rather than the anonymous ones.
	if featuresH != nil {
		r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/api/features", featuresH.List)
	}

	// Public status page (Phase C/D) — static HTML polling /api/health.
	r.Get("/status", func(w http.ResponseWriter, _ *http.Request) {
		b, err := os.ReadFile("deploy/status.html")
		if err != nil {
			http.Error(w, "status page missing", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=30")
		_, _ = w.Write(b)
	})

	if b.Legal != nil {
		r.Get("/api/legal/{doc}", b.Legal.Serve)
	}

	// Crawler-facing HTML for link previews. nginx rewrites social-network user
	// agents from /problems/hello to /meta/problems/hello; browsers never reach
	// these routes. The static "problems" child is registered so it wins over
	// the catch-all, which serves every other route's site default card.
	if b.Meta != nil {
		r.Get("/meta", b.Meta.Page)
		r.Get("/meta/*", b.Meta.Page)
		r.Get("/meta/problems/{slug}", b.Meta.Problem)
	}

	// Public sitemap for crawlers. Built from the same public-visibility rules
	// the problem/contest APIs apply, so it never advertises an auth-gated URL.
	if b.Sitemap != nil {
		r.Get("/sitemap.xml", func(w http.ResponseWriter, req *http.Request) {
			handler.ServeSitemapSections(b.Sitemap(req.Context()), w, req)
		})
	}

	if b.ContestFeed != nil {
		r.Get("/feed/contests.atom", func(w http.ResponseWriter, req *http.Request) {
			items, err := b.ContestFeed(req.Context())
			handler.ServeAtomFeed(items, err,
				"AIOJ — Contests", "Recent and upcoming contests on AIOJ",
				"http://localhost:8081/feed/contests.atom",
				"http://localhost:8081/feed/contests.atom", w, req)
		})
	}

	if b.CommentFeed != nil {
		r.Get("/feed/comments.atom", func(w http.ResponseWriter, req *http.Request) {
			items, err := b.CommentFeed(req.Context())
			handler.ServeAtomFeed(items, err,
				"AIOJ — Comments", "The latest comments on AIOJ",
				"http://localhost:8081/feed/comments.atom",
				"http://localhost:8081/feed/comments.atom", w, req)
		})
	}

	if b.Feed != nil {
		r.Get("/feed/problems.atom", func(w http.ResponseWriter, req *http.Request) {
			items, err := b.Feed(req.Context())
			handler.ServeAtomFeed(items, err,
				"AIOJ — New Problems", "The latest problems added to AIOJ",
				"http://localhost:8081/feed/problems.atom",
				"http://localhost:8081/feed/problems.atom", w, req)
		})
	}

	if b.BlogFeed != nil {
		r.Get("/feed/blog.atom", func(w http.ResponseWriter, req *http.Request) {
			items, err := b.BlogFeed(req.Context())
			handler.ServeAtomFeed(items, err,
				"AIOJ — Blog", "The latest posts from AIOJ",
				"http://localhost:8081/feed/blog.atom",
				"http://localhost:8081/feed/blog.atom", w, req)
		})
	}

	// Optional auth: the global search returns problem rows, and a generated
	// contestant account must not be able to enumerate the archive through it.
	r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/api/search", searchH.Search)
}
