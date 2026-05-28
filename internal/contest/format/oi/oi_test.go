package oi

import (
	"testing"

	"github.com/tahsinarafat/aioj/internal/contest/format"
)

func TestOIFormat_Name(t *testing.T) {
	f := &OIFormat{config: DefaultConfig()}
	if f.Name() != "oi" {
		t.Errorf("Name() = %v, want %v", f.Name(), "oi")
	}
}

func TestOIFormat_ScoreProblem(t *testing.T) {
	tests := []struct {
		name        string
		submissions []format.Submission
		wantScore   float64
		wantSolved  bool
	}{
		{
			name:        "no submissions",
			submissions: nil,
			wantScore:   0,
			wantSolved:  false,
		},
		{
			name: "single perfect score",
			submissions: []format.Submission{
				{Score: 100},
			},
			wantScore:  100,
			wantSolved: true,
		},
		{
			name: "multiple submissions take max",
			submissions: []format.Submission{
				{Score: 50},
				{Score: 80},
				{Score: 70},
			},
			wantScore:  80,
			wantSolved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &OIFormat{config: Config{MaxScorePerProblem: 100}}
			ctx := format.ScoringContext{
				Problem:     format.Problem{ID: 1, Index: "A"},
				Submissions: tt.submissions,
			}

			result, err := f.ScoreProblem(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Score != tt.wantScore {
				t.Errorf("Score = %v, want %v", result.Score, tt.wantScore)
			}
			if result.Solved != tt.wantSolved {
				t.Errorf("Solved = %v, want %v", result.Solved, tt.wantSolved)
			}
		})
	}
}

func TestOIFormat_RankParticipants(t *testing.T) {
	tests := []struct {
		name         string
		participants []format.ParticipantScore
		wantFirst    int64
	}{
		{
			name: "rank by total score",
			participants: []format.ParticipantScore{
				{UserID: 1, TotalScore: 200},
				{UserID: 2, TotalScore: 300},
				{UserID: 3, TotalScore: 150},
			},
			wantFirst: 2,
		},
		{
			name: "tiebreak by penalty",
			participants: []format.ParticipantScore{
				{UserID: 1, TotalScore: 200, TotalPenalty: 50},
				{UserID: 2, TotalScore: 200, TotalPenalty: 30},
			},
			wantFirst: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &OIFormat{config: DefaultConfig()}
			ranks := f.RankParticipants(tt.participants)

			if ranks[0].Score.UserID != tt.wantFirst {
				t.Errorf("first place = %d, want %d", ranks[0].Score.UserID, tt.wantFirst)
			}
		})
	}
}
