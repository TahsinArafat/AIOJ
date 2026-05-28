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
