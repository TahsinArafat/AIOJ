package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
)

func (b *routerBuilder) mountAuth(r chi.Router) {
	authH := b.Auth
	// Mint the double-submit cookie on demand: the SPA is served by nginx, so a
	// cold visitor on /login or /register has no csrf cookie before its first
	// POST. Public + safe method, so the CSRF middleware sets the cookie here.
	r.Get("/api/auth/csrf", authH.CSRFToken)
	r.With(b.rl.StrictAuth).Post("/api/auth/register", authH.Register)
	r.With(b.rl.StrictAuth).Post("/api/auth/login", authH.Login)
	r.Post("/api/auth/refresh", authH.Refresh)
	// Unauthenticated and idempotent so a client with an expired access token
	// can still end its session.
	r.Post("/api/auth/logout", authH.Logout)
	r.With(b.rl.StrictAuth).Post("/api/auth/forgot-password", authH.ForgotPassword)
	r.With(b.rl.StrictAuth).Post("/api/auth/reset-password", authH.ResetPassword)
	if b.VerifyEmail != nil {
		r.Get("/api/auth/verify-email/{token}", b.VerifyEmail.Verify)
	}
	r.With(middleware.AuthMiddleware(b.jwt)).Post("/api/auth/verify-email/resend", authH.ResendVerification)
	if b.TwoFA != nil {
		r.With(middleware.AuthMiddleware(b.jwt)).Post("/api/auth/2fa/begin", b.TwoFA.Begin)
		r.With(middleware.AuthMiddleware(b.jwt)).Post("/api/auth/2fa/enable", b.TwoFA.Enable)
		r.With(middleware.AuthMiddleware(b.jwt)).Post("/api/auth/2fa/disable", b.TwoFA.Disable)
	}
	if b.TwoFAVerify != nil {
		r.With(b.rl.StrictAuth).Post("/api/auth/2fa/verify", b.TwoFAVerify.Verify)
	}
	if b.DevMail != nil {
		r.Get("/api/dev/mail", b.DevMail.List)
		r.Delete("/api/dev/mail", b.DevMail.Clear)
	}
	if b.AuthMethods != nil {
		// Public: the login and register forms need this before there is a
		// session, to know which fields and provider buttons to render.
		r.Get("/api/auth/methods", b.AuthMethods.Public)
	}
	if b.OAuthStart != nil {
		r.Get("/api/auth/oauth/{provider}/start", b.OAuthStart.Start)
	}
	if b.OAuthCallback != nil {
		r.Get("/api/auth/oauth/{provider}/callback", b.OAuthCallback.Callback)
		// Second half of a first-time sign-in: the callback deliberately
		// returns a ticket instead of an account (see OAuthCallbackHandler),
		// so the person chooses the username before the row exists.
		r.With(b.rl.StrictAuth).Post("/api/auth/oauth/complete", b.OAuthCallback.CompleteSignup)
		r.Get("/api/auth/oauth/username-available", b.OAuthCallback.UsernameAvailable)
	}

	// The judge's own language list. Public and unauthenticated because the IDE
	// and the submit forms render it before sign-in, and because it is the same
	// information a visitor gets by opening any submission. The SPA used to keep
	// its own hardcoded copy, so an admin edit in the panel never reached it.
	if b.PublicLanguages != nil {
		r.Get("/api/languages", b.PublicLanguages.List)
	}
}
