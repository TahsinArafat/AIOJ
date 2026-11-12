package handler

import (
	"context"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

/**
 * Spoiler control for contest problems.
 *
 * A problem that belongs to a contest which has not finished yet still shows its
 * statement, but difficulty, rating, tags, the hint and the solve counts give the
 * intended solution away — on Codeforces the tags and rating only appear once the
 * round is over. Contest problems here are usually ordinary archive problems, so
 * hiding the fields on the contest endpoint alone is not enough: the same problem
 * is served by /api/problems/{slug}, by the problem list and by search. Every
 * public read therefore goes through these helpers, so all of them agree.
 *
 * Who is exempt differs by surface, and that is the point:
 *
 *   - The public problem page exempts the problem's own authors, who wrote the
 *     statement and are not being tested by it.
 *   - The contest page exempts only admins. A contest's manager or judge is
 *     already signed in when checking their own round, so exempting them put the
 *     answer key on the exact page the competitors use. Their full view is the
 *     setter workspace.
 */

// redactProblem clears the fields that would reveal a problem's difficulty.
func redactProblem(p *model.Problem) {
	p.Difficulty = ""
	p.Rating = 0
	p.Tags = nil
	p.Hint = ""
	p.SubmissionCount = 0
	p.AcceptedCount = 0
}

// problemIsLive reports whether the problem belongs to a contest that has not
// finished yet. A lookup error is treated as "not live" so a database hiccup
// degrades to today's behaviour rather than hiding every problem.
func problemIsLive(ctx context.Context, problems store.ProblemStore, p *model.Problem) bool {
	live, err := problems.InUnreleasedContest(ctx, p.ID)
	return err == nil && live
}

// canSeeSpoilers reports whether the viewer authored or tests the problem, and so
// is not the audience the difficulty is being hidden from.
func canSeeSpoilers(ctx context.Context, problems store.ProblemStore, p *model.Problem, claims *auth.Claims) bool {
	if claims == nil {
		return false
	}
	if claims.Role == "admin" {
		return true
	}
	return problems.HasAccess(ctx, p.ID, claims.UserID, "owner", "co_author", "tester")
}

func isAdmin(claims *auth.Claims) bool {
	return claims != nil && claims.Role == "admin"
}

/**
 * Visibility control for a private problem.
 *
 * A problem with visible = false is a draft: it belongs to its authors only. The
 * gate below is the single answer to "may this caller read it at all", and every
 * handler that returns a private problem's statement, test data, translations,
 * permissions or editorial must go through it. It was repeated per-handler
 * before, and the copies that were never written are exactly the leaks this
 * file exists to close — /export shipped every test file of every slug to any
 * signed-in account for want of one call.
 *
 * Authorization is by permission row, never by role: there is no `setter`
 * bypass, so one setter cannot open another's draft. problems.created_by is
 * checked directly as well, because Create inserts the owner row but a row whose
 * permission entry was lost must still be reachable by the person who wrote it.
 */

// problemAccessLevels are the permission levels that may read a private problem.
var problemAccessLevels = []string{"owner", "co_author", "tester"}

/**
 * Archive access for generated contestant accounts.
 *
 * The "Onsite Teams" panel mints throwaway accounts for one round. They are
 * ordinary users rows with role = "contestant" and are auto-registered into
 * their own contest, so the contest surface already works for them. The public
 * archive is a different matter: an account handed out for a two-hour onsite
 * round has no business browsing every problem on the platform, and nothing
 * else marks it. This is the one place that decision lives.
 */
func isArchiveBlocked(claims *auth.Claims) bool {
	return claims != nil && claims.Role == model.RoleContestant
}

// canReadProblem reports whether claims may read p at all. It answers only the
// draft question; whether a *published* problem is currently hidden because its
// contest is still running is problemIsLive's business, not this one's.
func canReadProblem(ctx context.Context, problems store.ProblemStore, p *model.Problem, claims *auth.Claims) bool {
	// A contestant account is scoped to its own round and gets no archive reads,
	// even for a published problem. Checked before the visible shortcut, which
	// would otherwise let it through.
	if isArchiveBlocked(claims) {
		return false
	}
	if p.Visible {
		return true
	}
	if claims == nil {
		return false
	}
	if claims.Role == "admin" || p.CreatedBy == claims.UserID {
		return true
	}
	if problems.HasAccess(ctx, p.ID, claims.UserID, problemAccessLevels...) {
		return true
	}
	// A draft attached to a contest is readable by that contest's staff. Two
	// setters co-running one round is the ordinary case: one attaches the problem
	// and the other cannot open it, which is what "another judge cannot access the
	// problem" described. This grants nothing on the public archive — the answer
	// is scoped to this problem and requires a manager/judge/tester row on a round
	// that actually contains it.
	staffed, err := problems.StaffedByContest(ctx, p.ID, claims.UserID)
	return err == nil && staffed
}
