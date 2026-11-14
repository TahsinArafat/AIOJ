package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
)

func (b *routerBuilder) mountContestExtras(r chi.Router) {
	contestAdjH := b.ContestAdjustment
	registrationH := b.Registration
	onsiteH := b.Onsite
	onsiteBatchH := b.OnsiteBatch
	clarificationH := b.Clarification
	noticeH := b.Notice
	// Manual score corrections. Deliberately NOT inside the setter-workspace
	// group above: a correction is a judging action, and a judge of a round does
	// not necessarily have authoring access. The handler itself checks
	// manager|judge|admin against the specific contest.
	r.Route("/api/contests/{id}/adjustments", func(r chi.Router) {
		r.Use(b.gate(feature.KeyContests))
		r.Use(middleware.AuthMiddleware(b.jwt))
		r.Get("/", contestAdjH.List)
		r.Post("/", contestAdjH.Create)
		r.Delete("/{adjustmentId}", contestAdjH.Delete)
	})

	r.Route("/api/contests/{contestId}/clarifications", func(r chi.Router) {
		r.Use(b.gate(feature.KeyContests))
		r.Get("/", clarificationH.List)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Post("/", clarificationH.Create)
			r.Post("/{id}/answer", clarificationH.Answer)
			r.Post("/bulk-answer", clarificationH.AnswerMany)
		})
	})

	r.Route("/api/contests/{contestId}/notices", func(r chi.Router) {
		r.Use(b.gate(feature.KeyContests))
		r.Get("/", noticeH.List)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Post("/", noticeH.Create)
			r.Delete("/{id}", noticeH.Delete)
		})
	})

	r.Route("/api/contests/{id}/register", func(r chi.Router) {
		r.Use(b.gate(feature.KeyContests))
		r.Get("/", registrationH.CheckRegistration)
		r.With(middleware.AuthMiddleware(b.jwt)).Post("/", registrationH.Register)
		r.With(middleware.AuthMiddleware(b.jwt)).Delete("/", registrationH.Unregister)
	})

	// The roster is staff-only. It was mounted outside the authenticated group
	// above, so an anonymous caller could read every registrant's username for any
	// contest id — on a public round that is the full competitor list, handed out
	// without even signing in. The handler also re-checks: this route must not be
	// the only thing standing between a stranger and a roster.
	r.With(b.gate(feature.KeyContests), middleware.AuthMiddleware(b.jwt)).
		Get("/api/contests/{id}/registrations", registrationH.ListRegistrations)

	r.Route("/api/contests/{id}/onsite", func(r chi.Router) {
		r.Use(b.gate(feature.KeyContests))
		r.Use(middleware.AuthMiddleware(b.jwt))
		r.Get("/balloons", onsiteH.ListBalloons)
		r.Post("/balloons/{balloonId}/dispatch", onsiteH.DispatchBalloon)
		r.Post("/print", onsiteH.RequestPrint)
		r.Get("/prints", onsiteH.ListPrints)
		r.Post("/prints/{printId}/status", onsiteH.UpdatePrintStatus)
		r.Post("/generate", onsiteBatchH.GenerateBatch)
		r.Get("/users", onsiteBatchH.ListBatch)
		r.Delete("/users/{userId}", onsiteBatchH.DeleteUser)
	})

	// A public password login: it needs the same brute-force budget as
	// /api/auth/login.
	r.With(b.rl.StrictAuth, b.gate(feature.KeyContests)).Post("/api/contests/{id}/onsite/login", onsiteBatchH.LoginAsTeam)
}
