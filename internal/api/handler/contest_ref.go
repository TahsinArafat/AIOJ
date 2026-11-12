package handler

import (
	"context"
	"errors"
	"net/http"

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
