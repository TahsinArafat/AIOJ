package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
	"net/http"
)

func (b *routerBuilder) mountAI(r chi.Router) {
	mediaH := b.Optional.Media
	// POST /api/generate/problem (the synchronous, unvalidated generator) has been
	// removed. POST /api/ai/jobs is the only generation entry point: it queues
	// the work, runs it through the sandboxed harness, and requires the result to
	// pass validation before it can be applied to a problem.
	if b.Optional.AIGeneration != nil && b.AIEnabled {
		r.Route("/api/ai", func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Use(middleware.RequireRole("admin", "setter"))
			r.Use(b.gate(feature.KeyAIGeneration))
			r.Get("/connections", b.Optional.AIGeneration.Connections)
			r.Get("/brief-presets", b.Optional.AIGeneration.Presets)
			r.Get("/jobs", b.Optional.AIGeneration.ListActor)
			r.Post("/jobs", b.Optional.AIGeneration.Create)
			r.Get("/jobs/{id}", b.Optional.AIGeneration.Get)
			r.Post("/jobs/{id}/cancel", b.Optional.AIGeneration.Cancel)
			r.Post("/jobs/{id}/discard", b.Optional.AIGeneration.Discard)
			r.Post("/jobs/{id}/apply", b.Optional.AIGeneration.Apply)
			r.Post("/jobs/{id}/retry", b.Optional.AIGeneration.Retry)
			r.Get("/problems/{problemId}/jobs", b.Optional.AIGeneration.ListProblem)
		})
		// Admin-only job monitoring: list all jobs across all setters, force-cancel.
		r.Route("/api/admin/ai", func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Use(middleware.RequireRole("admin"))
			r.Use(b.gate(feature.KeyAIGeneration))
			r.Get("/jobs", b.Optional.AIGeneration.AdminList)
			r.Post("/jobs/{id}/cancel", b.Optional.AIGeneration.AdminCancel)
		})
	}

	// Serve uploads through the media handler, not http.FileServer: FileServer
	// lists the directory for /media/ and serves whatever happens to be in it.
	if mediaH != nil {
		r.With(b.gate(feature.KeyMediaUploads)).Handle("/media/*", http.HandlerFunc(mediaH.Serve))
	}
}
