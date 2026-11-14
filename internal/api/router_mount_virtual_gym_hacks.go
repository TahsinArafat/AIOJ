package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
)

func (b *routerBuilder) mountVirtualGymHacks(r chi.Router) {
	virtualH := b.Virtual
	gymH := b.Gym
	hackH := b.Hack
	statsH := b.Stats
	r.Route("/api/virtual", func(r chi.Router) {
		r.Use(b.gate(feature.KeyVirtualContests))
		r.Use(middleware.AuthMiddleware(b.jwt))
		r.Post("/start", virtualH.Start)
		r.Get("/status", virtualH.Status)
		r.Post("/{id}/complete", virtualH.Complete)
	})

	r.Route("/api/gym", func(r chi.Router) {
		r.Use(b.gate(feature.KeyGym))
		r.Get("/", gymH.List)
		r.Get("/{id}", gymH.GetByID)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Post("/", gymH.Create)
			r.Post("/{id}/solve", gymH.MarkSolved)
			r.Get("/{id}/solved", gymH.IsSolved)
		})
	})

	r.Route("/api/hacks", func(r chi.Router) {
		r.Use(b.gate(feature.KeyHacks))
		r.Use(middleware.AuthMiddleware(b.jwt))
		r.Post("/", hackH.SubmitHack)
		r.Get("/{id}", hackH.GetHack)
		r.Get("/contest/{contestId}", hackH.ListContestHacks)
		r.Get("/hackable/{contestId}/{problemId}", hackH.ListHackableSubmissions)
	})

	r.Route("/api/stats", func(r chi.Router) {
		r.Get("/platform", statsH.GetPlatformStats)
		r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/problems/{problemId}", statsH.GetProblemStats)
		r.With(middleware.AuthMiddleware(b.jwt)).Get("/me", statsH.GetUserStats)
	})
}
