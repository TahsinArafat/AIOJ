package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
)

func (b *routerBuilder) mountSocial(r chi.Router) {
	notifH := b.Notification
	groupH := b.Group
	teamH := b.Team
	blogH := b.Blog
	r.Route("/api/notifications", func(r chi.Router) {
		r.Use(b.gate(feature.KeyNotifications))
		r.Use(middleware.AuthMiddleware(b.jwt))
		r.Get("/", notifH.List)
		r.Get("/unread-count", notifH.UnreadCount)
		r.Post("/{id}/read", notifH.MarkAsRead)
		r.Post("/read-all", notifH.MarkAllAsRead)
		r.Get("/preferences", notifH.GetPreferences)
		r.Put("/preferences", notifH.UpdatePreferences)
	})

	r.Route("/api/groups", func(r chi.Router) {
		r.Use(b.gate(feature.KeyGroups))
		r.Get("/", groupH.List)
		r.With(middleware.AuthMiddleware(b.jwt)).Post("/join-code", groupH.JoinByCode)
		r.Get("/{id}", groupH.GetByID)
		r.Get("/{id}/members", groupH.GetMembers)
		r.Get("/{id}/contests", groupH.GetContests)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Post("/", groupH.Create)
			r.Get("/my", groupH.ListByUser)
			r.Put("/{id}", groupH.Update)
			r.Delete("/{id}", groupH.Delete)
			r.Post("/{id}/join", groupH.Join)
			r.Post("/{id}/leave", groupH.Leave)
			r.Post("/{id}/contests", groupH.AddContest)
			r.Delete("/{id}/contests/{contestId}", groupH.RemoveContest)
			r.Post("/{id}/invite", groupH.InviteMember)
			r.Post("/{id}/respond", groupH.RespondInviteRequest)
			r.Get("/{id}/pending", groupH.GetPendingMembers)
		})
	})

	r.Route("/api/teams", func(r chi.Router) {
		r.Use(b.gate(feature.KeyTeams))
		r.Get("/", teamH.List)
		r.Get("/{id}", teamH.GetByID)
		r.Get("/{id}/members", teamH.GetMembers)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Post("/", teamH.Create)
			r.Post("/{id}/join", teamH.Join)
			r.Post("/{id}/leave", teamH.Leave)
			r.Put("/{id}", teamH.UpdateTeam)
			r.Delete("/{id}", teamH.DeleteTeam)
			r.Post("/{id}/invite", teamH.InviteMember)
			r.Post("/{id}/request", teamH.RequestJoin)
			r.Post("/{id}/respond", teamH.RespondInviteRequest)
			r.Get("/{id}/pending", teamH.GetPendingMembers)
		})
	})

	r.Route("/api/blog", func(r chi.Router) {
		r.Use(b.gate(feature.KeyBlog))
		r.Get("/", blogH.ListPosts)
		r.Get("/{id}", blogH.GetPost)
		r.Get("/{type}/{id}/comments", blogH.GetComments)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Post("/", blogH.CreatePost)
			r.Put("/{id}", blogH.UpdatePost)
			r.Delete("/{id}", blogH.DeletePost)
			r.Get("/{id}/vote", blogH.GetUserVote)
			r.Post("/comments", blogH.CreateComment)
			r.Post("/vote", blogH.Vote)
		})
	})
}
