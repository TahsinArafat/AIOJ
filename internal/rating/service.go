package rating

import (
	"context"
	"fmt"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/notify"
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
	// notify is optional; a nil notifier is a no-op.
	notify *notify.Notifier
}

func NewService(rs store.RatingStore, us store.UserStore) *Service {
	return &Service{
		ratingStore: rs,
		userStore:   us,
	}
}

// WithNotifier attaches a notifier and returns the service, for call-site chaining.
func (s *Service) WithNotifier(n *notify.Notifier) *Service {
	s.notify = n
	return s
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

		if s.userStore != nil {
			profile, err := s.userStore.GetProfile(ctx, change.UserID)
			if err != nil {
				return err
			}
			if profile != nil {
				profile.Rating = change.NewRating
				if change.NewRating > profile.MaxRating {
					profile.MaxRating = change.NewRating
				}
				profile.ContestCount++
				if err := s.userStore.UpdateRating(ctx, change.UserID, profile.Rating, profile.MaxRating, profile.ContestCount); err != nil {
					return err
				}
			}
		}

		// A rating change is the one event every competitor wants told about, and
		// it is low volume: once per contest per user, not once per submission.
		// Notification preferences gate it (CategoryRating).
		s.notify.Notify(ctx, change.UserID, "rating_change",
			fmt.Sprintf("Rating %+d → %d", change.RatingChange, change.NewRating),
			fmt.Sprintf("You placed #%d. Your rating is now %d.", change.Rank, change.NewRating),
			"/rating-history", notify.CategoryRating)
	}

	return nil
}

// TeamRater is the slice of the team store team ratings need.
type TeamRater interface {
	GetMembers(ctx context.Context, teamID string) ([]model.TeamMember, error)
	UpdateRating(ctx context.Context, teamID string, newRating int) error
}

// TeamRegistrar lists the teams registered for a contest.
type TeamRegistrar interface {
	ListTeamRegistrations(ctx context.Context, contestID string) ([]model.TeamRegistration, error)
}

// userRatingReader reads a user's current rating.
type userRatingReader interface {
	GetProfile(ctx context.Context, userID string) (*model.UserProfile, error)
}

// RecalculateTeamRatings updates every team registered for a contest.
//
// teams.rating and max_rating were permanently 0: TeamStore.UpdateRating existed
// but nothing ever called it, and team_rating_history was never written, so team
// rankings sorted by a column that never changed. A team's rating is the mean of
// its members' ratings after the round — the simplest rule that is not arbitrary,
// and one that moves when the members' own ratings move.
//
// Only called for a published, rated contest (see CalculateRatings), so an
// unapproved draft can never move a team's standing.
func (s *Service) RecalculateTeamRatings(ctx context.Context, contestID string, teams TeamRater, regs TeamRegistrar, profiles userRatingReader) error {
	registered, err := regs.ListTeamRegistrations(ctx, contestID)
	if err != nil {
		return err
	}
	for _, reg := range registered {
		members, err := teams.GetMembers(ctx, reg.TeamID)
		if err != nil {
			return err
		}
		if len(members) == 0 {
			continue
		}
		sum, n := 0, 0
		for _, m := range members {
			p, err := profiles.GetProfile(ctx, m.UserID)
			if err != nil || p == nil || p.Rating == 0 {
				// An unrated member contributes nothing rather than pulling the
				// mean to zero, which would punish a team for fielding a new
				// account.
				continue
			}
			sum += p.Rating
			n++
		}
		if n == 0 {
			continue
		}
		if err := teams.UpdateRating(ctx, reg.TeamID, sum/n); err != nil {
			return err
		}
	}
	return nil
}
