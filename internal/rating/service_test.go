package rating

import (
	"context"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

type mockRatingStore struct {
	histories []model.RatingHistory
}

func (m *mockRatingStore) CreateHistory(_ context.Context, h *model.RatingHistory) error {
	m.histories = append(m.histories, *h)
	return nil
}

func (m *mockRatingStore) GetByUser(_ context.Context, _ string, _ int) ([]model.RatingHistory, error) {
	return m.histories, nil
}

func (m *mockRatingStore) GetByContest(_ context.Context, _ string) ([]model.RatingHistory, error) {
	return m.histories, nil
}

func (m *mockRatingStore) GetLatestByUser(_ context.Context, _ string) (*model.RatingHistory, error) {
	if len(m.histories) > 0 {
		return &m.histories[len(m.histories)-1], nil
	}
	return nil, nil
}

func TestRatingService_CalculateContestRatings(t *testing.T) {
	service := NewService(nil, nil)

	standings := []ContestStanding{
		{UserID: "user1", Rank: 1, OldRating: 1500},
		{UserID: "user2", Rank: 2, OldRating: 1400},
		{UserID: "user3", Rank: 3, OldRating: 1600},
	}

	changes := service.CalculateContestRatings(standings)

	if len(changes) != 3 {
		t.Fatalf("Expected 3 changes, got %d", len(changes))
	}

	if changes[0].RatingChange <= 0 {
		t.Error("Winner should gain rating")
	}

	if changes[2].RatingChange >= 0 {
		t.Error("Last place should lose rating")
	}
}

func TestService_ApplyContestRatings(t *testing.T) {
	mock := &mockRatingStore{}
	service := NewService(mock, nil)

	changes := []model.RatingChange{
		{UserID: "user1", OldRating: 1500, NewRating: 1600, RatingChange: 100, Rank: 1},
		{UserID: "user2", OldRating: 1400, NewRating: 1450, RatingChange: 50, Rank: 2},
	}

	ctx := context.Background()
	err := service.ApplyContestRatings(ctx, "contest-1", changes)
	if err != nil {
		t.Fatalf("ApplyContestRatings failed: %v", err)
	}

	if len(mock.histories) != 2 {
		t.Fatalf("Expected 2 histories, got %d", len(mock.histories))
	}
}

// A round rated for a window still carries everyone on its board. The entrants
// outside it are dropped, and the survivors are re-ranked so they are not scored
// as though the ineligible rows had beaten them.
func TestRatedFieldDropsOutsideWindowAndReranks(t *testing.T) {
	standings := []ContestStanding{
		{UserID: "a", Rank: 1, OldRating: 1800},
		{UserID: "b", Rank: 2, OldRating: 1300},
		{UserID: "c", Rank: 3, OldRating: 1200},
		{UserID: "d", Rank: 4, OldRating: 900},
	}

	field := RatedField(standings, 0, 1599)

	if len(field) != 3 {
		t.Fatalf("expected 3 rated entrants, got %d: %+v", len(field), field)
	}
	if field[0].UserID != "b" || field[0].Rank != 1 {
		t.Errorf("first rated entrant = %s at rank %d, want b at rank 1", field[0].UserID, field[0].Rank)
	}
	if field[2].UserID != "d" || field[2].Rank != 3 {
		t.Errorf("last rated entrant = %s at rank %d, want d at rank 3", field[2].UserID, field[2].Rank)
	}
}

// An unrated account is eligible for a window that starts above zero: 0 is "no
// rating yet", and the calculator already reads it as DefaultRating.
func TestRatedFieldTreatsUnratedAsDefaultRating(t *testing.T) {
	standings := []ContestStanding{
		{UserID: "newcomer", Rank: 1, OldRating: 0},
		{UserID: "expert", Rank: 2, OldRating: 2100},
	}

	field := RatedField(standings, 1200, 1599)

	if len(field) != 1 || field[0].UserID != "newcomer" {
		t.Fatalf("expected only the newcomer in a 1200-1599 window, got %+v", field)
	}
}

// Ties survive the re-rank: two entrants on the same board rank keep the same
// rank, and the next entrant is placed after them rather than at the next index.
func TestRatedFieldKeepsTies(t *testing.T) {
	standings := []ContestStanding{
		{UserID: "a", Rank: 1, OldRating: 1500},
		{UserID: "b", Rank: 1, OldRating: 1500},
		{UserID: "c", Rank: 3, OldRating: 1500},
	}

	field := RatedField(standings, 0, 9999)

	want := []int{1, 1, 3}
	for i, rank := range want {
		if field[i].Rank != rank {
			t.Errorf("field[%d] rank = %d, want %d", i, field[i].Rank, rank)
		}
	}
}

// The first round a user ever contests has no history to read, which the store
// reports as 0. The delta must be computed against the starting rating, or the
// recorded change is a climb from nothing.
func TestCalculateContestRatingsUsesDefaultRatingForNewcomer(t *testing.T) {
	service := NewService(nil, nil)

	changes := service.CalculateContestRatings([]ContestStanding{
		{UserID: "newcomer", Rank: 1, OldRating: 0},
	})

	got := changes[0]
	if got.OldRating != DefaultRating {
		t.Fatalf("old rating = %d, want %d", got.OldRating, DefaultRating)
	}
	if got.OldRating+got.RatingChange != got.NewRating {
		t.Fatalf("change %+d does not take %d to %d", got.RatingChange, got.OldRating, got.NewRating)
	}
	if got.RatingChange >= DefaultRating {
		t.Fatalf("delta = %d, which is a climb from zero rather than a first rating", got.RatingChange)
	}
}
