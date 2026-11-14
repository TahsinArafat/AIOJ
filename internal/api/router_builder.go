package api

import (
	"net/http"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
)

// routerBuilder carries what every mount function needs: the wiring, the JWT
// manager, the rate limiter, and the per-route feature gate.
//
// It exists because the registrations used to sit inline in a single 893-line
// NewRouter, where adding a route meant editing a function that also did
// dependency unpacking, middleware assembly and every other domain's routing.
// Each domain now has its own mount<Domain> method in its own file, and they all
// take this one value.
//
// The Deps fields are reached through the embedded *Deps rather than repeated
// here, so there is still exactly one place that lists what the router may
// depend on — Deps itself.
type routerBuilder struct {
	*Deps
	jwt *auth.JWTManager
	rl  *middleware.RateLimiter
}

// gate is the per-route feature gate. It is a no-op when no resolver was wired
// (router unit tests build a Deps by hand), so a missing resolver fails open
// instead of panicking the whole server.
func (b *routerBuilder) gate(key string) func(http.Handler) http.Handler {
	if b.FeatureResolver == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return middleware.FeatureGate(b.jwt, b.FeatureResolver, key)
}

// base is the middleware every request passes through, in order.
//
// Order matters: RealIP must run before the rate limiter, which keys its buckets
// on the client address. (It used to run after, so every request behind nginx was
// bucketed by its ephemeral port and nothing was limited.) chi's RealIP is
// replaced by the trusted-proxy-aware version so a direct client cannot spoof
// X-Forwarded-For.
func (b *routerBuilder) base() []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		middleware.SecurityHeaders(),
		chiMiddleware.RequestID,
		middleware.RealIPFromEnv(),
		middleware.CSRF(b.CSRFSecret),
		middleware.RateLimit(b.rl, "/api/health", "/api/ws"),
		middleware.Logging,
		middleware.SentryRecover,
		chiMiddleware.Recoverer,
	}
}
