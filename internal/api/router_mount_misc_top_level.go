package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
)

func (b *routerBuilder) mountMiscTopLevel(r chi.Router) {
	adminH := b.Admin
	wsManager := b.WS
	ratingH := b.Rating
	rankingsH := b.Rankings
	remoteLangH := b.RemoteLang
	r.Get("/api/remote-languages/{platform}", remoteLangH.ListByPlatform)

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(b.jwt))
		r.Post("/api/auth/setter-apply", adminH.ApplySetter)
		r.Get("/api/auth/setter-status", adminH.GetSetterStatus)
	})

	r.Get("/api/ws", wsManager.Handle)

	r.Route("/api/rating", func(r chi.Router) {
		r.Get("/user/{userId}", ratingH.GetByUser)
		r.Get("/contest/{contestId}", ratingH.GetByContest)
	})

	r.With(b.gate(feature.KeyRankings)).Get("/api/rankings", rankingsH.List)
}
