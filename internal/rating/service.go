package rating

import (
	"context"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type ContestStanding struct {
	UserID    string
	Rank      int
	OldRating int
	Username  string
}

type Service struct {
	ratingStore store.RatingStore
	userStore   store.UserStore
}

func NewService(rs store.RatingStore, us store.UserStore) *Service {
	return &Service{
		ratingStore: rs,
		userStore:   us,
	}
}

// CalculateContestRatings computes rating changes for all participants
func (s *Service) CalculateContestRatings(standings []ContestStanding) []model.RatingChange {
	participants := len(standings)
	changes := make([]model.RatingChange, 0, participants)

	for _, standing := range standings {
		newRating := CalculateRating(standing.OldRating, standing.Rank, participants)
		ratingChange := CalculateRatingChange(standing.OldRating, newRating)

		changes = append(changes, model.RatingChange{
			UserID:       standing.UserID,
			Username:     standing.Username,
			OldRating:    standing.OldRating,
			NewRating:    newRating,
			RatingChange: ratingChange,
			Rank:         standing.Rank,
			Color:        model.GetColor(newRating),
		})
	}

	return changes
}

// ApplyContestRatings saves rating changes to database
func (s *Service) ApplyContestRatings(ctx context.Context, contestID string, changes []model.RatingChange) error {
	for _, change := range changes {
		h := &model.RatingHistory{
			UserID:       change.UserID,
			ContestID:    contestID,
			OldRating:    change.OldRating,
			NewRating:    change.NewRating,
			Rank:         change.Rank,
			RatingChange: change.RatingChange,
		}

		if err := s.ratingStore.CreateHistory(ctx, h); err != nil {
			return err
		}
	}

	return nil
}
