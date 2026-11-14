package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
)

func (b *routerBuilder) mountAdmin(r chi.Router) {
	problemH := b.Problem
	contestH := b.Contest
	adminH := b.Admin
	botAccountH := b.BotAccount
	settingsH := b.Settings
	featuresH := b.Features
	langAdminH := b.LangAdmin
	remoteLangH := b.RemoteLang
	adminSubH := b.AdminSub
	backupH := b.Backup
	r.Route("/api/admin", func(r chi.Router) {
		// A backup download is a plain browser navigation (an <a download>), which
		// cannot set an Authorization header. This is the only route group that
		// accepts ?token=; everywhere else it would leak into access logs.
		r.Use(middleware.AuthMiddleware(b.jwt, "/api/admin/backups/"))
		r.Use(middleware.RequireRole("admin"))
		if b.Optional.AuditLog != nil {
			am := &middleware.AuditRecorder{Store: b.Optional.AuditLog.Audit}
			r.Use(am.Middleware)
		}
		r.Get("/users", adminH.ListUsers)
		// Declared before the {id} patterns: chi matches static segments first,
		// but keeping "bulk" adjacent to the listing makes it obvious that the
		// collection has a batch endpoint.
		r.Post("/users/bulk", adminH.BulkUpdate)
		r.Get("/users/{id}", adminH.GetUser)
		r.Put("/users/{id}/role", adminH.UpdateUserRole)
		r.Put("/users/{id}/password", adminH.ResetUserPassword)
		r.Post("/users/{id}/ban", adminH.BanUser)
		r.Delete("/users/{id}/ban", adminH.UnbanUser)
		r.Post("/users/{id}/verify-email", adminH.VerifyUserEmail)
		r.Delete("/users/{id}/verify-email", adminH.UnverifyUserEmail)
		r.Get("/setter-applications", adminH.ListSetterApps)
		r.Post("/setter-applications/{id}/review", adminH.ReviewSetterApp)

		// Authoring review queue: the admin side of "setters propose, admins
		// approve". Registered under /api/admin, so AuthMiddleware + admin role
		// from this group's middleware already apply.
		r.Get("/problem-review", problemH.ReviewQueue)
		r.Post("/problem-review/{slug}", problemH.ReviewProblem)
		r.Get("/contest-review", contestH.ReviewQueue)
		r.Post("/contest-review/{id}", contestH.ReviewContest)

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

		if b.Optional.AuthMethods != nil {
			r.Route("/auth-methods", func(r chi.Router) {
				r.Get("/", b.Optional.AuthMethods.AdminGet)
				r.Put("/", b.Optional.AuthMethods.AdminSet)
			})
		}

		// Feature gates are never themselves gated: locking the admin out of
		// the panel that reopens a feature would be unrecoverable without SQL.
		if featuresH != nil {
			r.Route("/features", func(r chi.Router) {
				r.Get("/", featuresH.AdminList)
				r.Put("/{key}", featuresH.AdminSet)
				r.Delete("/{key}", featuresH.AdminReset)
			})
		}

		if b.Optional.CDN != nil {
			r.Get("/cdn/status", b.Optional.CDN.Status)
			r.Post("/cdn/purge", b.Optional.CDN.Purge)
		}

		if b.Optional.AuditLog != nil {
			r.Get("/audit-log", b.Optional.AuditLog.List)
		}

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
			// Restores the shipped lang/<key>.yaml. POST because it is an action
			// with an effect, not a document the client supplies.
			r.Post("/{key}/reset", langAdminH.Reset)
		})

		r.Route("/remote-languages", func(r chi.Router) {
			r.Get("/{platform}", remoteLangH.ListByPlatform)
			r.Post("/", remoteLangH.Create)
			r.Put("/{id}", remoteLangH.Update)
			r.Delete("/{id}", remoteLangH.Delete)
			r.Post("/detect/{platform}", remoteLangH.AutoDetect)
			r.Post("/bulk", remoteLangH.BulkUpsert)
		})

		r.Route("/submissions", func(r chi.Router) {
			r.Get("/pending-remote", adminSubH.ListPendingRemote)
			r.Get("/failed", adminSubH.ListFailed)
			r.Post("/rejudge-bulk", adminSubH.RejudgeBulk)
			r.Post("/{id}/rejudge", adminSubH.Rejudge)
			r.Post("/{id}/refresh", adminSubH.ForceRefresh)
		})

		r.Route("/backups", func(r chi.Router) {
			r.Get("/", backupH.List)
			r.Post("/", backupH.Create)
			r.Post("/upload", backupH.Upload)
			r.Get("/{filename}", backupH.Download)
			r.Delete("/{filename}", backupH.Delete)
			r.Post("/{filename}/restore", backupH.Restore)
		})

		r.Route("/ai-models", func(r chi.Router) {
			r.Get("/", b.AIModel.List)
			r.Post("/", b.AIModel.Create)
			r.Post("/test", b.AIModel.TestConnection)
			r.Get("/{id}", b.AIModel.GetByID)
			r.Put("/{id}", b.AIModel.Update)
			r.Put("/{id}/toggle", b.AIModel.Toggle)
			r.Delete("/{id}", b.AIModel.Delete)
		})
	})
}
