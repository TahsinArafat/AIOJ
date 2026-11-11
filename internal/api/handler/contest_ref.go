package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/tahsinarafat/aioj/internal/auth"
	"github.com/tahsinarafat/aioj/internal/model"
)

var errContestNotFound = errors.New("contest not found")

// resolveContest turns any public contest reference (display ID, slug, or
// UUID) into the canonical contest used by UUID-keyed stores.
func resolveContest(ctx context.Context, rawID string, contests interface {
	GetByID(context.Context, string) (*model.Contest, error)
}) (*model.Contest, error) {
	contest, err := contests.GetByID(ctx, rawID)
	if err != nil {
		return nil, err
	}
	if contest == nil {
		return nil, errContestNotFound
	}
	return contest, nil
}

func respondContestLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, errContestNotFound) {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}
	http.Error(w, "internal error", http.StatusInternalServerError)
}

// contestAccess is the slice of the contest store canReadContest needs. Narrow on
// purpose: gym, virtual and group handlers all need the same answer, and each
// currently hand-rolls its own version (or forgets to).
type contestAccess interface {
	HasAccess(ctx context.Context, contestID, userID string, requiredLevels ...string) bool
}

// canReadContest reports whether claims may see this contest at all.
//
// A contest becomes public when it is published and has started. Before that it
// belongs to the people running it, plus its creator. Extracted because four
// handlers had hand-copied this same nested condition (contest.go:338, :924,
// :977, contest_problem.go:49) and the handlers that needed a fifth copy
// (gym, virtual, group) simply never wrote one — which is how a gym could be
// pointed at a private contest.
func canReadContest(ctx context.Context, contests contestAccess, c *model.Contest, claims *auth.Claims) bool {
	if c == nil {
		return false
	}
	// Published and started: this is public information.
	if c.Visible && !time.Now().Before(c.StartTime) {
		return true
	}
	if claims == nil {
		return false
	}
	if claims.Role == "admin" || c.CreatedBy == claims.UserID {
		return true
	}
	return contests.HasAccess(ctx, c.ID, claims.UserID, "manager", "judge", "tester")
}
