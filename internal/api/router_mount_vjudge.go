package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
)

func (b *routerBuilder) mountVJudge(r chi.Router) {
	vjudgeH := b.VJudge
	r.Route("/api/vjudge", func(r chi.Router) {
		r.Use(b.gate(feature.KeyVJudge))
		r.Use(middleware.AuthMiddleware(b.jwt))
		r.Get("/bots", vjudgeH.ListBots)
		r.Post("/submit", vjudgeH.Submit)
	})
}
