package handler

import (
	"context"

	"github.com/tahsinarafat/aioj/internal/model"
)

// userDataAggregator adapts existing stores to UserDataAggregator.
// submissionAggregateReader is what the user-data aggregator reads: a user's
// submissions, plus one row when it needs the problem. Two methods, not thirty.
type submissionAggregateReader interface {
	GetByID(ctx context.Context, id string) (*model.Submission, error)
	ListByUser(ctx context.Context, userID string, offset, limit int, problemID, contestID string) ([]model.Submission, int, error)
}

// userProfileReader is the pair of user reads the aggregator performs: the
// account row and its extended profile.
type userProfileReader interface {
	GetByID(ctx context.Context, id string) (*model.User, error)
	GetProfile(ctx context.Context, userID string) (*model.UserProfile, error)
}

type userDataAggregator struct {
	users    userProfileReader
	subStore submissionAggregateReader
}

func NewUserDataAggregator(users userProfileReader, subs submissionAggregateReader) UserDataAggregator {
	return &userDataAggregator{users: users, subStore: subs}
}

func (a *userDataAggregator) User(ctx context.Context, id string) (*model.User, error) {
	return a.users.GetByID(ctx, id)
}

func (a *userDataAggregator) Profile(ctx context.Context, id string) (*model.UserProfile, error) {
	return a.users.GetProfile(ctx, id)
}

func (a *userDataAggregator) SubmissionCount(ctx context.Context, id string) (int, error) {
	_, total, err := a.subStore.ListByUser(ctx, id, 0, 1, "", "")
	return total, err
}
