package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
)

func (b *routerBuilder) mountContests(r chi.Router) {
	submissionH := b.Submission
	contestH := b.Contest
	contestUnlockH := b.ContestUnlock
	contestProblemH := b.ContestProblem
	r.Route("/api/contests", func(r chi.Router) {
		r.Use(b.gate(feature.KeyContests))
		r.Get("/", contestH.List)
		r.Get("/formats", contestH.ListAvailableFormats)
		r.With(middleware.AuthMiddleware(b.jwt)).Get("/my", contestH.ListMyContests)
		r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/{id}", contestH.GetByID)
		r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/{id}/scoreboard", contestH.Scoreboard)
		r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/{id}/stats", contestH.ContestStats)
		r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/{contestId}/problems/{index}", contestProblemH.GetByIndex)
		// Unlock is a credential check, so it gets StrictAuth: it is the one contest
		// endpoint a stranger would hammer to guess a round's password, and bcrypt
		// alone does not stop an online guessing loop. Mirrors the onsite-login
		// route further down.
		r.With(middleware.AuthMiddleware(b.jwt), b.rl.StrictAuth).Post("/{id}/unlock", contestUnlockH.Unlock)
		// The booklet is opened by a browser navigation: public for a published
		// round, staff-only for a draft. A navigation cannot set an Authorization
		// header, so this prefix accepts ?token= — the same facade the backup
		// download uses. Without it a setter could not fetch their own draft's
		// booklet, which is exactly what broke.
		r.With(middleware.OptionalAuthMiddleware(b.jwt, "/api/contests/")).Get("/{id}/pdf", contestH.DownloadPDF)
		// ICPC-style event feed for an external scorer. Authorized by an
		// operator-configured shared token, not a user session, so it is
		// deliberately outside the JWT machinery.
		r.Get("/{id}/feed", contestH.Feed)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			// Contest authoring and management belong to the setter workspace.
			r.Use(b.gate(feature.KeySetterWorkspace))
			r.Post("/", contestH.Create)
			r.Post("/educational", contestH.CreateEducational)
			r.Put("/{id}", contestH.Update)
			r.Delete("/{id}", contestH.Delete)
			// Propose a round for admin approval. Publishing itself is admin-only
			// and lives under /api/admin.
			r.Post("/{id}/submit", contestH.SubmitForReview)
			r.Get("/{id}/permissions", contestH.ListPermissions)
			r.Post("/{id}/permissions", contestH.AddPermission)
			r.Delete("/{id}/permissions/{userId}", contestH.RemovePermission)
			r.Post("/{id}/problems", contestH.AddProblem)
			r.Put("/{id}/problems/{problemId}", contestH.UpdateProblem)
			r.Delete("/{id}/problems/{problemId}", contestH.RemoveProblem)
			r.Get("/{id}/submissions", submissionH.ListByContest)
			// Voiding and rejudging are not mounted here: they act on one
			// submission and live under /api/submissions, which is what lets the
			// round's tab and the problem's offer the same actions from the same
			// code.
			// Ratings are a ledger, so the endpoints are the ledger's operations
			// rather than one button: apply once, then recalculate or withdraw.
			// Recalculation replays every rated round after the one named, because
			// the algorithm folds — a single round cannot be recomputed alone.
			r.Post("/{id}/calculate-ratings", contestH.CalculateRatings)
			r.Post("/{id}/recalculate-ratings", contestH.RecalculateRatings)
			r.Post("/{id}/void-ratings", contestH.VoidRatings)
			r.Delete("/{id}/void-ratings", contestH.UnvoidRatings)
			r.Get("/{id}/rating-runs", contestH.ListRatingRuns)
			r.Get("/{id}/rating-exclusions", contestH.ListRatingExclusions)
			r.Post("/{id}/rating-exclusions", contestH.AddRatingExclusion)
			r.Delete("/{id}/rating-exclusions/{userId}", contestH.RemoveRatingExclusion)
		})
		r.With(middleware.AuthMiddleware(b.jwt)).Post("/{id}/register-team", contestH.RegisterTeam)
		// The roster is organizer information; the handler checks manager/judge.
		r.With(middleware.AuthMiddleware(b.jwt)).Get("/{id}/team-registrations", contestH.ListTeamRegistrations)
	})
}
