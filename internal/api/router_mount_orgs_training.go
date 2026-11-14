package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
)

func (b *routerBuilder) mountOrgsTraining(r chi.Router) {
	orgH := b.Org
	classH := b.Class
	trainingH := b.Training
	plagiarismH := b.Plagiarism
	r.Route("/api/organizations", func(r chi.Router) {
		r.Use(b.gate(feature.KeyOrganizations))
		r.Get("/", orgH.List)
		r.Get("/{id}", orgH.GetByID)
		r.Get("/{id}/members", orgH.GetMembers)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
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
		r.Use(b.gate(feature.KeyClasses))
		r.Get("/", classH.List)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Post("/", classH.Create)
		})
	})

	r.Route("/api/classes", func(r chi.Router) {
		r.Use(b.gate(feature.KeyClasses))
		r.Get("/{id}", classH.GetByID)
		r.Get("/{id}/members", classH.GetMembers)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Put("/{id}", classH.Update)
			r.Delete("/{id}", classH.Delete)
			r.Post("/join", classH.JoinByCode)
			r.Post("/{id}/leave", classH.Leave)
		})
	})

	r.Route("/api/training", func(r chi.Router) {
		r.Use(b.gate(feature.KeyTraining))
		r.Get("/", trainingH.List)
		r.Get("/{id}", trainingH.GetByID)
		r.Get("/{id}/enrollments", trainingH.GetEnrollments)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
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
		r.Use(b.gate(feature.KeyPlagiarism))
		r.Use(middleware.AuthMiddleware(b.jwt))
		r.Post("/check", plagiarismH.RunCheck)
		r.Get("/report", plagiarismH.GetReportByContest)
		r.Get("/report/{reportId}", plagiarismH.GetReport)
		r.Get("/report/{reportId}/pairs", plagiarismH.ListPairs)
		r.Put("/pairs/{pairId}", plagiarismH.UpdatePairStatus)
	})
}
