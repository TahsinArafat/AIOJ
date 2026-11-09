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
 * is served by /api/problems/{slug}. Every public read therefore goes through
 * these two helpers, so all of them agree on what a viewer may see.
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

// redactProblemListItem does the same for the list projection, which only carries
// difficulty and rating.
func redactProblemListItem(item *model.ProblemListItem) {
	item.Difficulty = ""
	item.Rating = 0
}

// canSeeSpoilers reports whether the viewer is trusted with the full problem
// regardless of any contest: admins, and the people who author or test it.
func canSeeSpoilers(ctx context.Context, problems store.ProblemStore, p *model.Problem, claims *auth.Claims) bool {
	if claims == nil {
		return false
	}
	if claims.Role == "admin" {
		return true
	}
	return problems.HasAccess(ctx, p.ID, claims.UserID, "owner", "co_author", "tester")
}

// hideSpoilers reports whether this viewer must be shown the problem without the
// metadata that gives its difficulty away.
func hideSpoilers(ctx context.Context, problems store.ProblemStore, p *model.Problem, claims *auth.Claims) bool {
	if canSeeSpoilers(ctx, problems, p, claims) {
		return false
	}
	locked, err := problems.InUnreleasedContest(ctx, p.ID)
	return err == nil && locked
}
