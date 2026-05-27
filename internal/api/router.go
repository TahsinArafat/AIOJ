package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/tahsinarafat/aioj/internal/api/handler"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
)

func NewRouter(
	authH *handler.AuthHandler,
	problemH *handler.ProblemHandler,
	submissionH *handler.SubmissionHandler,
	contestH *handler.ContestHandler,
	vjudgeH *handler.VJudgeHandler,
	adminH *handler.AdminHandler,
	wsManager *handler.WSManager,
	jwtManager *auth.JWTManager,
) http.Handler {
	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID, chiMiddleware.RealIP, middleware.Logging, chiMiddleware.Recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`{"status":"ok"}`)) })

	r.Post("/api/auth/register", authH.Register)
	r.Post("/api/auth/login", authH.Login)
	r.Post("/api/auth/refresh", authH.Refresh)

	r.Route("/api/problems", func(r chi.Router) {
		r.Get("/", problemH.List)
		r.Get("/{slug}", problemH.GetBySlug)
		r.Get("/{slug}/submissions", submissionH.ListByProblem)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", problemH.Create)
		})
	})

	r.Route("/api/submissions", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/", submissionH.Create)
		r.Get("/", submissionH.ListByUser)
		r.Get("/{id}", submissionH.GetByID)
	})

	r.Route("/api/contests", func(r chi.Router) {
		r.Get("/", contestH.List)
		r.Get("/{id}", contestH.GetByID)
		r.Get("/{id}/scoreboard", contestH.Scoreboard)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", contestH.Create)
		})
	})

	r.Route("/api/vjudge", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Get("/bots", vjudgeH.ListBots)
		r.Post("/submit", vjudgeH.Submit)
	})

	r.Route("/api/admin", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Use(middleware.RequireRole("admin"))
		r.Get("/users", adminH.ListUsers)
		r.Put("/users/{id}/role", adminH.UpdateUserRole)
		r.Get("/setter-applications", adminH.ListSetterApps)
		r.Post("/setter-applications/{id}/review", adminH.ReviewSetterApp)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/api/auth/setter-apply", adminH.ApplySetter)
		r.Get("/api/auth/setter-status", adminH.GetSetterStatus)
	})

	r.Get("/api/ws", wsManager.Handle)

	return r
}
