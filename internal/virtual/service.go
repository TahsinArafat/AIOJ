package virtual

import (
	"context"
	"errors"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

var ErrActiveVirtualExists = errors.New("you already have an active virtual contest")

type Service struct {
	virtualStore store.VirtualStore
}

func NewService(vs store.VirtualStore) *Service {
	return &Service{virtualStore: vs}
}

func (s *Service) StartVirtualContest(ctx context.Context, userID, contestID string, durationMinutes int) (*model.VirtualContest, error) {
	existing, _ := s.virtualStore.GetActiveByUser(ctx, userID)
	if existing != nil {
		return nil, ErrActiveVirtualExists
	}

	if durationMinutes <= 0 {
		durationMinutes = 120
	}

	v := &model.VirtualContest{
		OriginalContestID: contestID,
		UserID:            userID,
		StartedAt:         time.Now(),
		DurationMinutes:   durationMinutes,
		Status:            "active",
	}

	if err := s.virtualStore.Create(ctx, v); err != nil {
		return nil, err
	}

	return v, nil
}

func (s *Service) GetStatus(v *model.VirtualContest, now time.Time) model.VirtualStatus {
	endsAt := v.StartedAt.Add(time.Duration(v.DurationMinutes) * time.Minute)
	remaining := int(time.Until(endsAt).Minutes())
	if remaining < 0 {
		remaining = 0
	}

	return model.VirtualStatus{
		IsActive:      v.Status == "active" && remaining > 0,
		VirtualID:     v.ID,
		StartedAt:     &v.StartedAt,
		EndsAt:        &endsAt,
		RemainingMins: remaining,
	}
}

func (s *Service) CompleteContest(ctx context.Context, virtualID string) error {
	return s.virtualStore.Complete(ctx, virtualID)
}
