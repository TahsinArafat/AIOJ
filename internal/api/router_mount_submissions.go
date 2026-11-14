package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
)

func (b *routerBuilder) mountSubmissions(r chi.Router) {
	submissionH := b.Submission
	r.Route("/api/submissions", func(r chi.Router) {
		r.Use(b.gate(feature.KeySubmissions))
		// The site-wide status page is readable without an account, so it is
		// registered outside the authenticated group below.
		r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/all", submissionH.ListAll)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			r.Post("/", submissionH.Create)
			r.Post("/upsolving", submissionH.CreateUpsolving)
			r.Post("/run", submissionH.CustomRun)
			// One compile-and-execute per sample case under a single quota
			// token. The client used to fire N /run calls, and /run's burst is
			// 3, so any problem with more samples than that failed partway.
			r.Post("/run-batch", submissionH.BatchRun)
			r.Get("/", submissionH.ListByUser)
			r.Get("/{id}", submissionH.GetByID)
			r.Post("/{id}/retry-remote", submissionH.RetryRemote)
			r.Post("/{id}/recheck-remote", submissionH.RecheckRemote)
			// Organizer actions on submissions, and the only two that exist.
			// They hang off the submission rather than off a problem or a round
			// because they are one decision about one row: the caller names the
			// row (or the set), and the handler derives the authority from what
			// the row actually is. Scoping the URL by problem or by round only
			// produced two ways to say the same thing, and left the round's tab
			// without a rejudge because a round spans problems.
			r.With(b.gate(feature.KeySetterWorkspace)).Post("/rejudge", submissionH.Rejudge)
			// Voiding is an accusation that moves a public leaderboard, so which
			// roles may do it is an admin decision (Admin → Feature Gates: "Void
			// a submission") rather than a side effect of holding the setter
			// role. The handler then checks the caller answers for THIS row.
			r.With(b.gate(feature.KeySetterWorkspace), b.gate(feature.KeySkipSubmission)).
				Post("/{id}/skip", submissionH.SkipSubmission)
		})
	})
}
