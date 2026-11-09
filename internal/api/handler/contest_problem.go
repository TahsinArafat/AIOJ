package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// contestProblemStore is the slice of the contest store this handler needs.
// Narrower than store.ContestStore so the spoiler policy can be tested without
// standing up the whole store.
type contestProblemStore interface {
	GetByID(ctx context.Context, id string) (*model.Contest, error)
	HasAccess(ctx context.Context, contestID, userID string, requiredLevels ...string) bool
	GetContestProblemByIndex(ctx context.Context, contestID, index string) (*model.Problem, error)
	IsParticipant(ctx context.Context, contestID, userID string) (bool, error)
}

type ContestProblemHandler struct {
	contestStore contestProblemStore
	problemStore store.ProblemStore
}

func NewContestProblemHandler(cs contestProblemStore, ps store.ProblemStore) *ContestProblemHandler {
	return &ContestProblemHandler{
		contestStore: cs,
		problemStore: ps,
	}
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
	claims := middleware.GetUserClaims(r)

	if !contest.Visible || now.Before(contest.StartTime) {
		if claims == nil || (claims.Role != "admin" && !h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "tester")) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	problem, err := h.contestStore.GetContestProblemByIndex(r.Context(), contest.ID, index)
	if err != nil || problem == nil {
		http.Error(w, "problem not found in contest", http.StatusNotFound)
		return
	}

	isJudge := claims != nil && (claims.Role == "admin" || h.contestStore.HasAccess(r.Context(), contest.ID, claims.UserID, "manager", "judge"))

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
			if !isParticipant && claims.Role != "admin" {
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
