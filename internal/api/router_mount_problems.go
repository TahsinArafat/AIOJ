package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/feature"
)

func (b *routerBuilder) mountProblems(r chi.Router) {
	problemH := b.Problem
	problemI18nH := b.ProblemI18n
	submissionH := b.Submission
	testcaseH := b.Testcase
	langLimitH := b.LangLimit
	importH := b.Import
	mediaH := b.Optional.Media
	r.Route("/api/problems", func(r chi.Router) {
		r.Use(b.gate(feature.KeyProblems))
		// Optional auth: the archive itself is public, but a generated contestant
		// account must be refused it, and that needs the caller's role.
		// Anonymous and ordinary visitors are unaffected.
		r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/", problemH.List)
		r.Get("/tags", problemH.ListTags)
		r.With(middleware.AuthMiddleware(b.jwt)).Get("/my", problemH.ListMyProblems)
		// The contest problem picker searches a wider set than the archive: it
		// must also offer the caller's own unpublished drafts. Auth-gated
		// because the result is per-caller.
		r.With(middleware.AuthMiddleware(b.jwt)).Get("/selectable", problemH.ListSelectable)
		r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/{slug}", problemH.GetBySlug)

		// Per-problem statement translations (dmoj-style i18n). The handler
		// decides who may read them, because a draft's translations are as
		// private as the draft — so it needs the caller's identity even on the
		// read path, which plain public routing would not supply.
		r.Route("/{id}/i18n", func(r chi.Router) {
			r.With(middleware.OptionalAuthMiddleware(b.jwt)).Get("/", problemI18nH.List)
			r.With(middleware.AuthMiddleware(b.jwt)).Put("/{lang}", problemI18nH.Upsert)
			r.With(middleware.AuthMiddleware(b.jwt)).Delete("/{lang}", problemI18nH.Delete)
		})
		// A problem's submission list. `/` stays readable without an account;
		// `/manage` and `/rejudge` are the setter's own view and actions.
		r.Route("/{slug}/submissions", func(r chi.Router) {
			r.Get("/", submissionH.ListByProblem)
			r.Group(func(r chi.Router) {
				r.Use(middleware.AuthMiddleware(b.jwt))
				r.Use(b.gate(feature.KeySetterWorkspace))
				r.Get("/manage", submissionH.ListByProblemManage)
				r.Post("/rejudge", submissionH.RejudgeProblemSubmissions)
				// Voiding a submission is an accusation that moves a public
				// leaderboard, so which roles may do it is an admin decision
				// (Admin → Feature Gates: "Void a submission") rather than a
				// side effect of holding the setter role. b.gate() rejects a
				// disallowed role before the handler runs; the handler then
				// checks the caller is accountable for THIS submission. The
				// route is problem-scoped so the permission is always checked
				// against the problem that was actually answered.
				r.With(b.gate(feature.KeySkipSubmission)).Post("/{id}/skip", submissionH.SkipProblemSubmission)
			})
		})
		r.Get("/{slug}/language-limits", langLimitH.List)
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(b.jwt))
			// Everything below writes a problem, its testcases or its
			// permissions: that is the setter workspace, so the authoring gate
			// covers it as well as the UI.
			r.Use(b.gate(feature.KeySetterWorkspace))
			r.Post("/", problemH.Create)
			r.Put("/{slug}", problemH.Update)
			r.Delete("/{slug}", problemH.Delete)
			// Publishing is an admin decision: authors submit, admins review.
			r.Post("/{slug}/submit", problemH.SubmitForReview)
			r.Get("/{slug}/permissions", problemH.ListPermissions)
			r.Post("/{slug}/permissions", problemH.AddPermission)
			r.Delete("/{slug}/permissions/{userId}", problemH.RemovePermission)
			r.Post("/{slug}/testcases", testcaseH.Upload)
			// Run the problem's validator over its inputs. Authoring rights only.
			r.Post("/{slug}/validate-cases", testcaseH.ValidateCases)
			r.Get("/{slug}/testcases", testcaseH.List)
			r.Get("/{slug}/testcases/{name}", testcaseH.Get)
			r.Put("/{slug}/testcases/{name}", testcaseH.Save)
			r.Delete("/{slug}/testcases/{name}", testcaseH.Delete)
			r.Post("/{slug}/language-limits", langLimitH.Set)
			r.Delete("/{slug}/language-limits/{lang}", langLimitH.Delete)
			r.Get("/{slug}/export", problemH.Export)
			// Importing from another judge is its own admin-configurable
			// permission (feature "problem_import"), not an automatic right of
			// the setter workspace: pulling third-party problems into the set is
			// a curation decision. b.gate() is a no-op when no resolver is wired,
			// so router tests still build.
			r.With(b.gate(feature.KeyProblemImport)).Post("/import", importH.Import)
			r.With(b.gate(feature.KeyProblemImport)).Post("/import/codeforces", importH.ImportCodeforces)
			r.With(b.gate(feature.KeyProblemImport)).Post("/import/cses", importH.ImportCSES)
			r.With(b.gate(feature.KeyProblemImport)).Post("/import/atcoder", importH.ImportAtCoder)
			r.With(b.gate(feature.KeyProblemImport)).Post("/import/toph", importH.ImportToph)
			r.With(b.gate(feature.KeyProblemImport)).Post("/import/qoj", importH.ImportQOJ)
			r.Post("/{slug}/media", mediaH.Upload)
		})
	})
}
