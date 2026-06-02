package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/auth"
)

func NewRouter(d Deps, jwtManager *auth.JWTManager) http.Handler {
	authH := d.Auth
	problemH := d.Problem
	submissionH := d.Submission
	contestH := d.Contest
	contestProblemH := d.ContestProblem
	vjudgeH := d.VJudge
	adminH := d.Admin
	testcaseH := d.Testcase
	wsManager := d.WS
	ratingH := d.Rating
	registrationH := d.Registration
	virtualH := d.Virtual
	gymH := d.Gym
	hackH := d.Hack
	statsH := d.Stats
	notifH := d.Notification
	groupH := d.Group
	teamH := d.Team
	blogH := d.Blog
	editorialH := d.Editorial
	apiKeyH := d.APIKey
	webhookH := d.Webhook
	recommendationH := d.Recommendation
	rankingsH := d.Rankings
	usersH := d.Users
	searchH := d.Search
	langLimitH := d.LangLimit
	importH := d.Import
	orgH := d.Org
	classH := d.Class
	trainingH := d.Training
	plagiarismH := d.Plagiarism
	mediaH := d.Media
	onsiteH := d.Onsite
	onsiteBatchH := d.OnsiteBatch
	clarificationH := d.Clarification
	noticeH := d.Notice
	botAccountH := d.BotAccount
	settingsH := d.Settings
	langAdminH := d.LangAdmin
	remoteLangH := d.RemoteLang
	adminSubH := d.AdminSub
	r := chi.NewRouter()
	rl := middleware.NewRateLimiter()
	r.Use(middleware.RateLimit(rl, "/api/health", "/api/ws", "/metrics"), chiMiddleware.RequestID, chiMiddleware.RealIP, middleware.Logging, chiMiddleware.Recoverer)

	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(`{"status":"ok"}`)) })

	r.Get("/api/search", searchH.Search)

	r.Post("/api/auth/register", authH.Register)
	r.Post("/api/auth/login", authH.Login)
	r.Post("/api/auth/refresh", authH.Refresh)
	r.Post("/api/auth/forgot-password", authH.ForgotPassword)
	r.Post("/api/auth/reset-password", authH.ResetPassword)

	r.Get("/api/users/{username}", usersH.GetByUsername)

	r.Route("/api/problems", func(r chi.Router) {
		r.Get("/", problemH.List)
		r.Get("/tags", problemH.ListTags)
		r.With(middleware.OptionalAuthMiddleware(jwtManager)).Get("/{slug}", problemH.GetBySlug)
		r.Get("/{slug}/submissions", submissionH.ListByProblem)
		r.Get("/{slug}/language-limits", langLimitH.List)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", problemH.Create)
			r.Put("/{slug}", problemH.Update)
			r.Delete("/{slug}", problemH.Delete)
			r.Get("/{slug}/permissions", problemH.ListPermissions)
			r.Post("/{slug}/permissions", problemH.AddPermission)
			r.Delete("/{slug}/permissions/{userId}", problemH.RemovePermission)
			r.Post("/{slug}/testcases", testcaseH.Upload)
			r.Post("/{slug}/language-limits", langLimitH.Set)
			r.Delete("/{slug}/language-limits/{lang}", langLimitH.Delete)
			r.Get("/{slug}/export", problemH.Export)
			r.Post("/import", importH.Import)
			r.Post("/import/codeforces", importH.ImportCodeforces)
		r.Post("/import/cses", importH.ImportCSES)
			r.Post("/{slug}/media", mediaH.Upload)
		})
	})

	r.Route("/api/submissions", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/", submissionH.Create)
		r.Post("/upsolving", submissionH.CreateUpsolving)
		r.Post("/run", submissionH.CustomRun)
		r.Get("/", submissionH.ListByUser)
		r.Get("/{id}", submissionH.GetByID)
	})

	r.Route("/api/contests", func(r chi.Router) {
		r.Get("/", contestH.List)
		r.Get("/formats", contestH.ListAvailableFormats)
		r.With(middleware.OptionalAuthMiddleware(jwtManager)).Get("/{id}", contestH.GetByID)
		r.With(middleware.OptionalAuthMiddleware(jwtManager)).Get("/{id}/scoreboard", contestH.Scoreboard)
		r.With(middleware.OptionalAuthMiddleware(jwtManager)).Get("/{id}/stats", contestH.ContestStats)
		r.With(middleware.OptionalAuthMiddleware(jwtManager)).Get("/{contestId}/problems/{index}", contestProblemH.GetByIndex)
		r.With(middleware.OptionalAuthMiddleware(jwtManager)).Get("/{id}/pdf", contestH.DownloadPDF)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", contestH.Create)
			r.Post("/educational", contestH.CreateEducational)
			r.Put("/{id}", contestH.Update)
			r.Delete("/{id}", contestH.Delete)
			r.Get("/{id}/permissions", contestH.ListPermissions)
			r.Post("/{id}/permissions", contestH.AddPermission)
			r.Delete("/{id}/permissions/{userId}", contestH.RemovePermission)
			r.Post("/{id}/problems", contestH.AddProblem)
			r.Delete("/{id}/problems/{problemId}", contestH.RemoveProblem)
			r.Get("/{id}/submissions", submissionH.ListByContest)
		})
		r.Post("/{id}/calculate-ratings", contestH.CalculateRatings)
		r.Post("/{id}/register-team", contestH.RegisterTeam)
		r.Get("/{id}/team-registrations", contestH.ListTeamRegistrations)
	})

	r.Route("/api/contests/{contestId}/clarifications", func(r chi.Router) {
		r.Get("/", clarificationH.List)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", clarificationH.Create)
			r.Post("/{id}/answer", clarificationH.Answer)
		})
	})

	r.Route("/api/contests/{contestId}/notices", func(r chi.Router) {
		r.Get("/", noticeH.List)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", noticeH.Create)
			r.Delete("/{id}", noticeH.Delete)
		})
	})

	r.Route("/api/contests/{id}/register", func(r chi.Router) {
		r.Get("/", registrationH.CheckRegistration)
		r.With(middleware.AuthMiddleware(jwtManager)).Post("/", registrationH.Register)
		r.With(middleware.AuthMiddleware(jwtManager)).Delete("/", registrationH.Unregister)
	})

	r.Get("/api/contests/{id}/registrations", registrationH.ListRegistrations)

	r.Route("/api/contests/{id}/onsite", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Get("/balloons", onsiteH.ListBalloons)
		r.Post("/balloons/{balloonId}/dispatch", onsiteH.DispatchBalloon)
		r.Post("/print", onsiteH.RequestPrint)
		r.Get("/prints", onsiteH.ListPrints)
		r.Post("/prints/{printId}/status", onsiteH.UpdatePrintStatus)
		r.Post("/generate", onsiteBatchH.GenerateBatch)
		r.Get("/users", onsiteBatchH.ListBatch)
	})

	r.Post("/api/contests/{id}/onsite/login", onsiteBatchH.LoginAsTeam)

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

		r.Route("/bot-accounts", func(r chi.Router) {
			r.Get("/", botAccountH.List)
			r.Post("/", botAccountH.Create)
			r.Post("/test-login", botAccountH.TestLogin)
			r.Get("/{id}", botAccountH.GetByID)
			r.Put("/{id}", botAccountH.Update)
			r.Delete("/{id}", botAccountH.Delete)
		})

		r.Route("/settings", func(r chi.Router) {
			r.Get("/", settingsH.List)
			r.Post("/", settingsH.Set)
			r.Get("/{key}", settingsH.Get)
			r.Delete("/{key}", settingsH.Delete)
		})

		r.Route("/languages", func(r chi.Router) {
			r.Get("/", langAdminH.List)
			r.Post("/", langAdminH.Create)
			r.Get("/detect", langAdminH.Detect)
			r.Get("/templates", langAdminH.Templates)
			r.Get("/{key}", langAdminH.Get)
			r.Put("/{key}", langAdminH.Update)
			r.Delete("/{key}", langAdminH.Delete)
			r.Get("/{key}/raw", langAdminH.GetRaw)
			r.Put("/{key}/raw", langAdminH.UpdateRaw)
			r.Post("/{key}/test", langAdminH.Test)
		})

		r.Route("/remote-languages", func(r chi.Router) {
			r.Get("/{platform}", remoteLangH.ListByPlatform)
			r.Post("/", remoteLangH.Create)
			r.Put("/{id}", remoteLangH.Update)
			r.Delete("/{id}", remoteLangH.Delete)
		})

		r.Route("/submissions", func(r chi.Router) {
			r.Get("/pending-remote", adminSubH.ListPendingRemote)
			r.Post("/{id}/rejudge", adminSubH.Rejudge)
			r.Post("/{id}/refresh", adminSubH.ForceRefresh)
		})
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

	r.Get("/api/rankings", rankingsH.List)

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
		r.Get("/platform", statsH.GetPlatformStats)
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

	r.Route("/api/recommendations", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Get("/", recommendationH.GetRecommendations)
	})

	r.Route("/api/organizations", func(r chi.Router) {
		r.Get("/", orgH.List)
		r.Get("/{id}", orgH.GetByID)
		r.Get("/{id}/members", orgH.GetMembers)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", orgH.Create)
			r.Put("/{id}", orgH.Update)
			r.Delete("/{id}", orgH.Delete)
			r.Post("/{id}/join", orgH.Join)
			r.Post("/{id}/leave", orgH.Leave)
			r.Post("/{id}/members", orgH.AddMember)
			r.Delete("/{id}/members/{userId}", orgH.RemoveMember)
			r.Get("/my", orgH.MyOrganizations)
		})
	})

	r.Route("/api/organizations/{orgId}/classes", func(r chi.Router) {
		r.Get("/", classH.List)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", classH.Create)
		})
	})

	r.Route("/api/classes", func(r chi.Router) {
		r.Get("/{id}", classH.GetByID)
		r.Get("/{id}/members", classH.GetMembers)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Put("/{id}", classH.Update)
			r.Delete("/{id}", classH.Delete)
			r.Post("/join", classH.JoinByCode)
			r.Post("/{id}/leave", classH.Leave)
		})
	})

	r.Route("/api/training", func(r chi.Router) {
		r.Get("/", trainingH.List)
		r.Get("/{id}", trainingH.GetByID)
		r.Get("/{id}/enrollments", trainingH.GetEnrollments)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(jwtManager))
			r.Post("/", trainingH.Create)
			r.Put("/{id}", trainingH.Update)
			r.Delete("/{id}", trainingH.Delete)
			r.Post("/{id}/enroll", trainingH.Enroll)
			r.Delete("/{id}/enroll", trainingH.Unenroll)
			r.Get("/{id}/progress", trainingH.GetMyProgress)
			r.Post("/{id}/sections", trainingH.AddSection)
			r.Delete("/sections/{sectionId}", trainingH.DeleteSection)
			r.Post("/sections/{sectionId}/problems", trainingH.AddProblem)
			r.Delete("/problems/{problemId}", trainingH.RemoveProblem)
		})
	})

	r.Route("/api/contests/{contestId}/plagiarism", func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/check", plagiarismH.RunCheck)
		r.Get("/report", plagiarismH.GetReportByContest)
		r.Get("/report/{reportId}", plagiarismH.GetReport)
		r.Get("/report/{reportId}/pairs", plagiarismH.ListPairs)
		r.Put("/pairs/{pairId}", plagiarismH.UpdatePairStatus)
	})

	fileServer := http.FileServer(http.Dir("./media"))
	r.Handle("/media/*", http.StripPrefix("/media/", fileServer))

	return r
}
