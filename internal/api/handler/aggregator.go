package handler

import (
	"context"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

// userDataAggregator adapts existing stores to UserDataAggregator.
type userDataAggregator struct {
	users    store.UserStore
	subStore store.SubmissionStore
}

func NewUserDataAggregator(users store.UserStore, subs store.SubmissionStore) UserDataAggregator {
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
