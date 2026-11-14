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
	if b.Optional.Achievements != nil {
		r.With(middleware.AuthMiddleware(b.jwt)).Get("/api/users/{userId}/achievements", b.Optional.Achievements.List)
		r.With(middleware.AuthMiddleware(b.jwt)).Post("/api/users/{userId}/achievements/{code}", b.Optional.Achievements.Award)
	}
	if b.Optional.Friendships != nil {
		r.With(middleware.AuthMiddleware(b.jwt)).Post("/api/users/{username}/follow", b.Optional.Friendships.Follow)
		r.With(middleware.AuthMiddleware(b.jwt)).Delete("/api/users/{username}/follow", b.Optional.Friendships.Unfollow)
		r.Get("/api/users/{username}/follow/status", b.Optional.Friendships.Status)
		r.Get("/api/users/{username}/followers", b.Optional.Friendships.ListFollowers)
		r.Get("/api/users/{username}/following", b.Optional.Friendships.ListFollowing)
	}
	if b.Optional.UsersExport != nil {
		r.With(middleware.AuthMiddleware(b.jwt)).Get("/api/users/me/export", b.Optional.UsersExport.ExportMyData)
	}
	if b.Optional.UsersDeletion != nil {
		r.With(middleware.AuthMiddleware(b.jwt)).Delete("/api/users/me", b.Optional.UsersDeletion.DeleteAccount)
	}
	r.With(middleware.AuthMiddleware(b.jwt)).Get("/api/users/profile/edit", usersH.GetProfile)
	r.With(middleware.AuthMiddleware(b.jwt)).Put("/api/users/profile/edit", usersH.UpdateProfile)
	r.With(middleware.AuthMiddleware(b.jwt)).Put("/api/users/profile/password", usersH.UpdatePassword)
	r.With(middleware.AuthMiddleware(b.jwt)).Get("/api/users/profile/invites", usersH.GetUserPendingInvites)
}
