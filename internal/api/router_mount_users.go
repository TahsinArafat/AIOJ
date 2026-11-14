package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
)

func (b *routerBuilder) mountUsers(r chi.Router) {
	usersH := b.Users
	r.Get("/api/users/{username}", usersH.GetByUsername)
	r.Get("/api/users/{username}/submissions", usersH.GetUserSubmissions)
	r.Get("/api/users/{username}/blogs", usersH.GetUserBlogs)
	r.Get("/api/users/{username}/comments", usersH.GetUserComments)

	// Phase D: achievements + social graph
	if b.Achievements != nil {
		r.With(middleware.AuthMiddleware(b.jwt)).Get("/api/users/{userId}/achievements", b.Achievements.List)
		r.With(middleware.AuthMiddleware(b.jwt)).Post("/api/users/{userId}/achievements/{code}", b.Achievements.Award)
	}
	if b.Friendships != nil {
		r.With(middleware.AuthMiddleware(b.jwt)).Post("/api/users/{username}/follow", b.Friendships.Follow)
		r.With(middleware.AuthMiddleware(b.jwt)).Delete("/api/users/{username}/follow", b.Friendships.Unfollow)
		r.Get("/api/users/{username}/follow/status", b.Friendships.Status)
		r.Get("/api/users/{username}/followers", b.Friendships.ListFollowers)
		r.Get("/api/users/{username}/following", b.Friendships.ListFollowing)
	}
	if b.UsersExport != nil {
		r.With(middleware.AuthMiddleware(b.jwt)).Get("/api/users/me/export", b.UsersExport.ExportMyData)
	}
	if b.UsersDeletion != nil {
		r.With(middleware.AuthMiddleware(b.jwt)).Delete("/api/users/me", b.UsersDeletion.DeleteAccount)
	}
	r.With(middleware.AuthMiddleware(b.jwt)).Get("/api/users/profile/edit", usersH.GetProfile)
	r.With(middleware.AuthMiddleware(b.jwt)).Put("/api/users/profile/edit", usersH.UpdateProfile)
	r.With(middleware.AuthMiddleware(b.jwt)).Put("/api/users/profile/password", usersH.UpdatePassword)
	r.With(middleware.AuthMiddleware(b.jwt)).Get("/api/users/profile/invites", usersH.GetUserPendingInvites)
}
