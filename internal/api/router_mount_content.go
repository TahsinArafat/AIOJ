package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
)

func (b *routerBuilder) mountContent(r chi.Router) {
	editorialH := b.Editorial
	apiKeyH := b.APIKey
	webhookH := b.Webhook
	recommendationH := b.Recommendation
	r.Route("/api/editorials", func(r chi.Router) {
		r.Use(b.gate(feature.KeyEditorials))
		r.Get("/", editorialH.List)
		r.Get("/{id}", editorialH.GetByID)
		r.Get("/problem/{problemId}", editorialH.GetByProblem)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Post("/", editorialH.Create)
		})
	})

	r.Route("/api/keys", func(r chi.Router) {
		r.Use(b.gate(feature.KeyAPIKeys))
		r.Use(middleware.AuthMiddleware(b.jwt))
		r.Get("/", apiKeyH.List)
		r.Post("/", apiKeyH.Create)
		r.Delete("/{id}", apiKeyH.Delete)
	})

	r.Route("/api/webhooks", func(r chi.Router) {
		r.Use(b.gate(feature.KeyWebhooks))
		r.Use(middleware.AuthMiddleware(b.jwt))
		r.Get("/", webhookH.List)
		r.Post("/", webhookH.Create)
		r.Delete("/{id}", webhookH.Delete)
	})

	r.Route("/api/recommendations", func(r chi.Router) {
		r.Use(b.gate(feature.KeyRecommendations))
		r.Use(middleware.AuthMiddleware(b.jwt))
		r.Get("/", recommendationH.GetRecommendations)
	})
}
