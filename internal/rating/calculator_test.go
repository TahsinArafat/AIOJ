package rating

import (
	"testing"
)

func TestCalculateEloRating(t *testing.T) {
	tests := []struct {
		name         string
		oldRating    int
		rank         int
		participants int
		expected     int
	}{
		{
			name:         "first contest, middle rank",
			oldRating:    0,
			rank:         50,
			participants: 100,
			expected:     1500, // Starting rating
		},
		{
			name:         "improvement from low rating",
			oldRating:    1200,
			rank:         10,
			participants: 100,
			expected:     1350, // Should increase significantly
		},
		{
			name:         "decline from high rating",
			oldRating:    2000,
			rank:         90,
			participants: 100,
			expected:     1900, // Should decrease
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRating(tt.oldRating, tt.rank, tt.participants)
			// Allow +/- 50 for Elo variance
			if result < tt.expected-50 || result > tt.expected+50 {
				t.Errorf("CalculateRating(%d, %d, %d) = %d, want ~%d",
					tt.oldRating, tt.rank, tt.participants, result, tt.expected)
			}
		})
	}
}

func TestCalculateRatingChange(t *testing.T) {
	change := CalculateRatingChange(1500, 1600)
	if change != 100 {
		t.Errorf("CalculateRatingChange(1500, 1600) = %d, want 100", change)
	}
}
