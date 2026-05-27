package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/tahsinarafat/aioj/internal/api/handler"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
)

func NewRouter(authH *handler.AuthHandler, problemH *handler.ProblemHandler, jwtManager *auth.JWTManager) http.Handler {
	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID, chiMiddleware.RealIP, middleware.Logging, chiMiddleware.Recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	r.Post("/api/auth/register", authH.Register)
	r.Post("/api/auth/login", authH.Login)
	r.Post("/api/auth/refresh", authH.Refresh)

	r.Route("/api/problems", func(r chi.Router) {
		r.Get("/", problemH.List)
		r.Get("/{slug}", problemH.GetBySlug)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", problemH.Create)
		})
	})

	return r
}
