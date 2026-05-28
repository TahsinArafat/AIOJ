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
	testcaseH *handler.TestcaseHandler,
	wsManager *handler.WSManager,
	jwtManager *auth.JWTManager,
	ratingH *handler.RatingHandler,
	registrationH *handler.RegistrationHandler,
	virtualH *handler.VirtualHandler,
	gymH *handler.GymHandler,
	hackH *handler.HackHandler,
	statsH *handler.StatsHandler,
	notifH *handler.NotificationHandler,
	groupH *handler.GroupHandler,
	teamH *handler.TeamHandler,
	blogH *handler.BlogHandler,
	editorialH *handler.EditorialHandler,
	apiKeyH *handler.APIKeyHandler,
	webhookH *handler.WebhookHandler,
) http.Handler {
	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID, chiMiddleware.RealIP, middleware.Logging, chiMiddleware.Recoverer)

	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`{"status":"ok"}`)) })

	r.Post("/api/auth/register", authH.Register)
	r.Post("/api/auth/login", authH.Login)
	r.Post("/api/auth/refresh", authH.Refresh)

	r.Route("/api/problems", func(r chi.Router) {
		r.Get("/", problemH.List)
		r.Get("/tags", problemH.ListTags)
		r.With(middleware.OptionalAuthMiddleware(jwtManager)).Get("/{slug}", problemH.GetBySlug)
		r.Get("/{slug}/submissions", submissionH.ListByProblem)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", problemH.Create)
			r.Put("/{slug}", problemH.Update)
			r.Delete("/{slug}", problemH.Delete)
			r.Get("/{slug}/permissions", problemH.ListPermissions)
			r.Post("/{slug}/permissions", problemH.AddPermission)
			r.Delete("/{slug}/permissions/{userId}", problemH.RemovePermission)
			r.Post("/{slug}/testcases", testcaseH.Upload)
		})
	})

	r.Route("/api/submissions", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/", submissionH.Create)
		r.Post("/upsolving", submissionH.CreateUpsolving)
		r.Get("/", submissionH.ListByUser)
		r.Get("/{id}", submissionH.GetByID)
	})

	r.Route("/api/contests", func(r chi.Router) {
		r.Get("/", contestH.List)
		r.With(middleware.OptionalAuthMiddleware(jwtManager)).Get("/{id}", contestH.GetByID)
		r.With(middleware.OptionalAuthMiddleware(jwtManager)).Get("/{id}/scoreboard", contestH.Scoreboard)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", contestH.Create)
			r.Post("/educational", contestH.CreateEducational)
			r.Get("/{id}/permissions", contestH.ListPermissions)
			r.Post("/{id}/permissions", contestH.AddPermission)
			r.Delete("/{id}/permissions/{userId}", contestH.RemovePermission)
		})
		r.Post("/{id}/calculate-ratings", contestH.CalculateRatings)
	})

	r.Route("/api/contests/{id}/register", func(r chi.Router) {
		r.Get("/", registrationH.CheckRegistration)
		r.With(middleware.AuthMiddleware(jwtManager)).Post("/", registrationH.Register)
		r.With(middleware.AuthMiddleware(jwtManager)).Delete("/", registrationH.Unregister)
	})

	r.Get("/api/contests/{id}/registrations", registrationH.ListRegistrations)

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

	r.Route("/api/rating", func(r chi.Router) {
		r.Get("/user/{userId}", ratingH.GetByUser)
		r.Get("/contest/{contestId}", ratingH.GetByContest)
	})

	r.Route("/api/virtual", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/start", virtualH.Start)
		r.Get("/status", virtualH.Status)
		r.Post("/{id}/complete", virtualH.Complete)
	})

	r.Route("/api/gym", func(r chi.Router) {
		r.Get("/", gymH.List)
		r.Get("/{id}", gymH.GetByID)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", gymH.Create)
			r.Post("/{id}/solve", gymH.MarkSolved)
		})
	})

	r.Route("/api/hacks", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/", hackH.SubmitHack)
		r.Get("/{id}", hackH.GetHack)
		r.Get("/contest/{contestId}", hackH.ListContestHacks)
		r.Get("/hackable/{contestId}/{problemId}", hackH.ListHackableSubmissions)
	})

	r.Route("/api/stats", func(r chi.Router) {
		r.Get("/problems/{problemId}", statsH.GetProblemStats)
		r.With(middleware.AuthMiddleware(jwtManager)).Get("/me", statsH.GetUserStats)
	})

	r.Route("/api/notifications", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Get("/", notifH.List)
		r.Get("/unread-count", notifH.UnreadCount)
		r.Post("/{id}/read", notifH.MarkAsRead)
		r.Post("/read-all", notifH.MarkAllAsRead)
		r.Get("/preferences", notifH.GetPreferences)
		r.Put("/preferences", notifH.UpdatePreferences)
	})

	r.Route("/api/groups", func(r chi.Router) {
		r.Get("/", groupH.List)
		r.Get("/{id}", groupH.GetByID)
		r.Get("/{id}/members", groupH.GetMembers)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", groupH.Create)
			r.Post("/{id}/join", groupH.Join)
			r.Post("/{id}/leave", groupH.Leave)
			r.Post("/{id}/contests", groupH.AddContest)
		})
	})

	r.Route("/api/teams", func(r chi.Router) {
		r.Get("/", teamH.List)
		r.Get("/{id}", teamH.GetByID)
		r.Get("/{id}/members", teamH.GetMembers)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", teamH.Create)
			r.Post("/{id}/join", teamH.Join)
			r.Post("/{id}/leave", teamH.Leave)
		})
	})

	r.Route("/api/blog", func(r chi.Router) {
		r.Get("/", blogH.ListPosts)
		r.Get("/{id}", blogH.GetPost)
		r.Get("/{type}/{id}/comments", blogH.GetComments)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", blogH.CreatePost)
			r.Post("/comments", blogH.CreateComment)
			r.Post("/vote", blogH.Vote)
		})
	})

	r.Route("/api/editorials", func(r chi.Router) {
		r.Get("/", editorialH.List)
		r.Get("/{id}", editorialH.GetByID)
		r.Get("/problem/{problemId}", editorialH.GetByProblem)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", editorialH.Create)
		})
	})

	r.Route("/api/keys", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Get("/", apiKeyH.List)
		r.Post("/", apiKeyH.Create)
		r.Delete("/{id}", apiKeyH.Delete)
	})

	r.Route("/api/webhooks", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Get("/", webhookH.List)
		r.Post("/", webhookH.Create)
		r.Delete("/{id}", webhookH.Delete)
	})

	return r
}
