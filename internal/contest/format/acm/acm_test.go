package acm

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/contest/format"
)

func TestACMFormat_Name(t *testing.T) {
	f := &ACMFormat{config: DefaultConfig()}
	if f.Name() != "acm" {
		t.Errorf("Name() = %v, want %v", f.Name(), "acm")
	}
}

func TestACMFormat_ScoreProblem(t *testing.T) {
	start := time.Now()
	tests := []struct {
		name         string
		config       Config
		submissions  []format.Submission
		wantSolved   bool
		wantAttempts int
		wantPenalty  int
	}{
		{
			name:         "no submissions",
			config:       DefaultConfig(),
			submissions:  nil,
			wantSolved:   false,
			wantAttempts: 0,
			wantPenalty:  0,
		},
		{
			name:   "first try AC at 30 minutes",
			config: DefaultConfig(),
			submissions: []format.Submission{
				{Status: "AC", CreatedAt: start.Add(30 * time.Minute)},
			},
			wantSolved:   true,
			wantAttempts: 1,
			wantPenalty:  30,
		},
		{
			name:   "AC after 3 wrong at 75 minutes",
			config: DefaultConfig(),
			submissions: []format.Submission{
				{Status: "WA", CreatedAt: start.Add(10 * time.Minute)},
				{Status: "WA", CreatedAt: start.Add(20 * time.Minute)},
				{Status: "WA", CreatedAt: start.Add(30 * time.Minute)},
				{Status: "AC", CreatedAt: start.Add(75 * time.Minute)},
			},
			wantSolved:   true,
			wantAttempts: 4,
			wantPenalty:  75 + 3*20, // 135
		},
		{
			name:   "all wrong no AC",
			config: DefaultConfig(),
			submissions: []format.Submission{
				{Status: "WA", CreatedAt: start.Add(10 * time.Minute)},
				{Status: "TLE", CreatedAt: start.Add(20 * time.Minute)},
			},
			wantSolved:   false,
			wantAttempts: 2,
			wantPenalty:  0,
		},
		{
			name:   "custom penalty per wrong",
			config: Config{PenaltyPerWrong: 30, TimePenalty: true},
			submissions: []format.Submission{
				{Status: "WA", CreatedAt: start.Add(10 * time.Minute)},
				{Status: "AC", CreatedAt: start.Add(50 * time.Minute)},
			},
			wantSolved:   true,
			wantAttempts: 2,
			wantPenalty:  50 + 1*30, // 80
		},
		{
			name:   "no time penalty",
			config: Config{PenaltyPerWrong: 20, TimePenalty: false},
			submissions: []format.Submission{
				{Status: "WA", CreatedAt: start.Add(10 * time.Minute)},
				{Status: "AC", CreatedAt: start.Add(50 * time.Minute)},
			},
			wantSolved:   true,
			wantAttempts: 2,
			wantPenalty:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &ACMFormat{config: tt.config}
			ctx := format.ScoringContext{
				ContestID:           1,
				SubmissionStartTime: start,
				Problem:            format.Problem{ID: 1, Index: "A"},
				Submissions:        tt.submissions,
			}

			result, err := f.ScoreProblem(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Solved != tt.wantSolved {
				t.Errorf("Solved = %v, want %v", result.Solved, tt.wantSolved)
			}
			if result.Attempts != tt.wantAttempts {
				t.Errorf("Attempts = %v, want %v", result.Attempts, tt.wantAttempts)
			}
			if result.Penalty != tt.wantPenalty {
				t.Errorf("Penalty = %v, want %v", result.Penalty, tt.wantPenalty)
			}
		})
	}
}

func TestACMFormat_RankParticipants(t *testing.T) {
	tests := []struct {
		name         string
		participants []format.ParticipantScore
		wantRanks    []int
	}{
		{
			name: "simple ordering",
			participants: []format.ParticipantScore{
				{UserID: 1, TotalSolved: 3, TotalPenalty: 300},
				{UserID: 2, TotalSolved: 4, TotalPenalty: 400},
				{UserID: 3, TotalSolved: 2, TotalPenalty: 200},
			},
			wantRanks: []int{2, 1, 3},
		},
		{
			name: "tie on solved and penalty",
			participants: []format.ParticipantScore{
				{UserID: 1, TotalSolved: 3, TotalPenalty: 300},
				{UserID: 2, TotalSolved: 3, TotalPenalty: 300},
			},
			wantRanks: []int{1, 1},
		},
		{
			name: "all zero solved",
			participants: []format.ParticipantScore{
				{UserID: 1, TotalSolved: 0, TotalPenalty: 0},
				{UserID: 2, TotalSolved: 0, TotalPenalty: 0},
				{UserID: 3, TotalSolved: 5, TotalPenalty: 250},
			},
			wantRanks: []int{2, 2, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &ACMFormat{config: DefaultConfig()}
			ranks := f.RankParticipants(tt.participants)

			for _, r := range ranks {
				expectedPos := tt.wantRanks[r.Score.UserID-1]
				if r.Position != expectedPos {
					t.Errorf("participant %d: position = %d, want %d", r.Score.UserID, r.Position, expectedPos)
				}
			}
		})
	}
}

func TestACMFormat_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  json.RawMessage
		wantErr bool
	}{
		{
			name:    "valid default",
			config:  json.RawMessage(`{"penalty_per_wrong":20,"time_penalty":true}`),
			wantErr: false,
		},
		{
			name:    "negative penalty",
			config:  json.RawMessage(`{"penalty_per_wrong":-5}`),
			wantErr: true,
		},
		{
			name:    "invalid json",
			config:  json.RawMessage(`{invalid}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &ACMFormat{config: DefaultConfig()}
			err := f.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
