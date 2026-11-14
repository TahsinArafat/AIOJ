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

// userRatingStore is the pair of user operations the rating job performs: read
// the current rating from the profile, write the new one back. Narrower than
// store.UserStore so the job cannot reach the ban, role or password fields.
type userRatingStore interface {
	GetProfile(ctx context.Context, userID string) (*model.UserProfile, error)
	UpdateRating(ctx context.Context, userID string, rating, maxRating, contestCount int) error
}

type Service struct {
	ratingStore store.RatingStore
	userStore   userRatingStore
	// notify is optional; a nil notifier is a no-op.
	notify *notify.Notifier
}

func NewService(rs store.RatingStore, us userRatingStore) *Service {
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

// effectiveRating is the rating a participant enters a round with.
//
// A user with no rating history has no rating yet, which the store reports as 0.
// That is "unrated", not "rated zero": CalculateRating already substitutes
// DefaultRating for it. The eligibility window and the delta have to read the
// same number, because computing the change against 0 records a newcomer's first
// rating as a gain of ~1500 and draws their graph as a climb from nothing.
func effectiveRating(r int) int {
	if r == 0 {
		return DefaultRating
	}
	return r
}

// RatedField narrows a ranked field to the participants a round is rated for.
//
// A round can be rated for a rating window (rated_for_min..rated_for_max) while
// its standings include everyone who submitted, and the calculator reads only
// rank and field size. Leaving the ineligible rows in place would count them in
// the field size and leave holes in the ranks, so an eligible entrant finishing
// 7th of 20 would be scored as though six people had beaten them when only three
// had. Re-ranking compactly keeps ties: participants sharing a board rank keep
// sharing a rank here.
//
// The input must be in board order, which is rank order.
func RatedField(standings []ContestStanding, min, max int) []ContestStanding {
	field := make([]ContestStanding, 0, len(standings))
	boardRank := 0
	for _, s := range standings {
		r := effectiveRating(s.OldRating)
		if r < min || r > max {
			continue
		}
		rank := len(field) + 1
		if len(field) > 0 && s.Rank == boardRank {
			rank = field[len(field)-1].Rank
		}
		boardRank = s.Rank
		s.Rank = rank
		field = append(field, s)
	}
	return field
}

// CalculateContestRatings computes rating changes for all participants
func (s *Service) CalculateContestRatings(standings []ContestStanding) []model.RatingChange {
	participants := len(standings)
	changes := make([]model.RatingChange, 0, participants)

	for _, standing := range standings {
		oldRating := effectiveRating(standing.OldRating)
		newRating := CalculateRating(oldRating, standing.Rank, participants)
		ratingChange := CalculateRatingChange(oldRating, newRating)

		changes = append(changes, model.RatingChange{
			UserID:       standing.UserID,
			Username:     standing.Username,
			OldRating:    oldRating,
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
