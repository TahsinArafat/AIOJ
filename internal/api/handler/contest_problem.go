package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// contestProblemStore is the slice of the contest store this handler needs.
// Narrower than store.ContestStore so the spoiler policy can be tested without
// standing up the whole store.
type contestProblemStore interface {
	GetByID(ctx context.Context, id string) (*model.Contest, error)
	HasAccess(ctx context.Context, contestID, userID string, requiredLevels ...string) bool
	CheckGroupRestriction(ctx context.Context, contestID, userID string) (bool, error)
	GetContestProblemByIndex(ctx context.Context, contestID, index string) (*model.Problem, error)
	IsParticipant(ctx context.Context, contestID, userID string) (bool, error)
}

type ContestProblemHandler struct {
	contestHandlerAccess
	contestStore contestProblemStore
	problemStore store.ProblemStore
}

// WithGrants attaches the password-entitlement store and rebuilds the access
// policy so it can consult it. Returns the handler for call-site chaining.
func (h *ContestProblemHandler) WithGrants(gs contestGrantChecker) *ContestProblemHandler {
	h.rebuildWithGrants(h.contestStore, gs)
	return h
}

func NewContestProblemHandler(cs contestProblemStore, ps store.ProblemStore) *ContestProblemHandler {
	h := &ContestProblemHandler{
		contestStore: cs,
		problemStore: ps,
	}
	// The store interface here is the narrow one, which carries both questions the
	// policy asks (HasAccess and CheckGroupRestriction), so no extra dependency is
	// needed to build the policy.
	h.access = newContestAccessPolicy(cs, nil)
	return h
}

func (h *ContestProblemHandler) GetByIndex(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "contestId")
	index := chi.URLParam(r, "index")

	contest, err := h.contestStore.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	now := time.Now()

	// The statement is the surface that leaked most: this check used to be the
	// visible/started rule only, so a group-restricted or password-protected round
	// handed its problems to anyone who knew the id. 404 rather than 403 so a draft
	// is not confirmed to exist.
	//
	// Its own surface, which a setter can widen independently of the problem list:
	// a teaser round publishes which problems exist without publishing what they
	// ask, and the reverse (statements open, list closed) is not useful but is not
	// harmful either — the list is a subset of what a statement reveals.
	viewer, ok := h.requireContestSurfaceAccess(w, r, contest, model.SurfaceStatement, true)
	if !ok {
		return
	}
	claims := viewer.claims

	problem, err := h.contestStore.GetContestProblemByIndex(r.Context(), contest.ID, index)
	if err != nil || problem == nil {
		http.Error(w, "problem not found in contest", http.StatusNotFound)
		return
	}

	// Membership in the contest is what made this readable, and the store read
	// deliberately skips p.visible so a private problem still opens during the
	// round. That grant must not outlive the round: otherwise attaching a draft
	// to any contest makes it readable through this endpoint forever, which is a
	// way around the fact that an unpublished problem is nobody else's business.
	// Once the contest is over the draft goes back to being private, and only its
	// authors and admins keep access.
	//
	// canReadDraft is computed once and reused below: someone who may read the
	// draft at all (its author, a collaborator, an admin) is not a participant to
	// be gated on, so the upsolving rule must not turn them away either.
	canReadDraft := canReadProblem(r.Context(), h.problemStore, problem, claims)
	if now.After(contest.EndTime) && !canReadDraft {
		http.Error(w, "problem not found in contest", http.StatusNotFound)
		return
	}

	isJudge := viewer.decision.IsJudge()

	// Difficulty, rating, tags, the hint and the solve counts stay hidden until
	// every contest using this problem is over, the way Codeforces reveals tags
	// and rating only after the round.
	//
	// Only admins are exempt. Being the contest's manager, judge or tester — or
	// the problem's author — is deliberately NOT a reason to see them here: that
	// is the button a setter presses while already logged in, so exempting them
	// meant the person checking the round saw the spoilers on the very page the
	// competitors use. Their full view lives in the setter workspace.
	seesSpoilers := isAdmin(claims) || !problemIsLive(r.Context(), h.problemStore, problem)
	if !seesSpoilers {
		redactProblem(problem)
	}

	if now.After(contest.EndTime) && !problem.Visible {
		if !contest.UpsolvingEnabled {
			if claims == nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			isParticipant, _ := h.contestStore.IsParticipant(r.Context(), contest.ID, claims.UserID)
			if !isParticipant && claims.Role != "admin" && !canReadDraft {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			respondJSON(w, http.StatusOK, map[string]interface{}{
				"problem":            problem,
				"contest":            contest,
				"can_submit":         false,
				"upsolving_disabled": true,
				"spoilers_hidden":    !seesSpoilers,
			})
			return
		}
	}

	canSubmit := true
	if now.After(contest.EndTime) && !problem.Visible && !contest.UpsolvingEnabled {
		canSubmit = false
	}

	statementHidden := contest.StatementHidden && !isJudge

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"problem":          problem,
		"contest":          contest,
		"can_submit":       canSubmit,
		"statement_hidden": statementHidden,
		"is_judge":         isJudge,
		"spoilers_hidden":  !seesSpoilers,
	})
}
