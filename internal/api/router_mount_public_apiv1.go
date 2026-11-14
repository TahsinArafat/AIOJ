package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
)

func (b *routerBuilder) mountPublicAPIv1(r chi.Router) {
	problemH := b.Problem
	contestH := b.Contest
	statsH := b.Stats
	// Public API v1: authenticated by X-API-Key instead of a JWT.
	//
	// Deliberately read-only and small. Keys could be minted since migration
	// 000014 and authenticated nothing — GetByHash had no caller and no
	// middleware read the header — so the feature was keys-only. A narrow,
	// obviously-safe surface is the right first step; widening it later is easy
	// and shrinking a wide one is not.
	if b.Optional.APIKeys != nil && b.Optional.APIKeyUsers != nil {
		r.Route("/api/v1", func(r chi.Router) {
			r.Use(middleware.APIKeyAuth(b.Optional.APIKeys, b.Optional.APIKeyUsers))
			r.Get("/problems", problemH.List)
			r.Get("/problems/tags", problemH.ListTags)
			r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/problems/{slug}", problemH.GetBySlug)
			r.Get("/contests", contestH.List)
			r.Get("/contests/{id}", contestH.GetByID)
			r.Get("/stats/me", statsH.GetUserStats)
		})
	}
}
