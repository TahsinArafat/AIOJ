package codeforces

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/contest/format"
)

func TestCodeforcesFormat_Name(t *testing.T) {
	f := &CodeforcesFormat{config: DefaultConfig()}
	if f.Name() != "codeforces" {
		t.Errorf("Name() = %v, want %v", f.Name(), "codeforces")
	}
}

func TestCodeforcesFormat_ScoreProblem(t *testing.T) {
	start := time.Now()
	contestDuration := 2 * time.Hour

	tests := []struct {
		name        string
		config      Config
		problemIdx  string
		submissions []format.Submission
		wantSolved  bool
		minScore    float64
		maxScore    float64
	}{
		{
			name:        "no submissions",
			config:      DefaultConfig(),
			problemIdx:  "A",
			submissions: nil,
			wantSolved:  false,
			minScore:    0,
			maxScore:    0,
		},
		{
			name:       "first try AC early",
			config:     DefaultConfig(),
			problemIdx: "A",
			submissions: []format.Submission{
				{Status: "AC", CreatedAt: start.Add(30 * time.Minute)},
			},
			wantSolved: true,
			minScore:   400,
			maxScore:   500,
		},
		{
			name:       "AC with wrong attempts",
			config:     DefaultConfig(),
			problemIdx: "B",
			submissions: []format.Submission{
				{Status: "WA", CreatedAt: start.Add(10 * time.Minute)},
				{Status: "WA", CreatedAt: start.Add(20 * time.Minute)},
				{Status: "AC", CreatedAt: start.Add(60 * time.Minute)},
			},
			wantSolved: true,
			minScore:   300,
			maxScore:   1000,
		},
		{
			name:       "minimum score floor",
			config:     Config{InitialScores: []int{1000}, DecayFactor: 250, MinScoreRatio: 0.35, WrongSubmissionPenalty: 50},
			problemIdx: "A",
			submissions: []format.Submission{
				{Status: "WA", CreatedAt: start.Add(10 * time.Minute)},
				{Status: "WA", CreatedAt: start.Add(20 * time.Minute)},
				{Status: "WA", CreatedAt: start.Add(30 * time.Minute)},
				{Status: "WA", CreatedAt: start.Add(40 * time.Minute)},
				{Status: "WA", CreatedAt: start.Add(50 * time.Minute)},
				{Status: "AC", CreatedAt: start.Add(110 * time.Minute)},
			},
			wantSolved: true,
			minScore:   350,
			maxScore:   350,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CodeforcesFormat{config: tt.config}
			ctx := format.ScoringContext{
				ContestID:           1,
				ContestDuration:     contestDuration,
				SubmissionStartTime: start,
				Problem:            format.Problem{ID: 1, Index: tt.problemIdx},
				Submissions:        tt.submissions,
			}

			result, err := f.ScoreProblem(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Solved != tt.wantSolved {
				t.Errorf("Solved = %v, want %v", result.Solved, tt.wantSolved)
			}

			if result.Solved {
				if result.Score < tt.minScore || result.Score > tt.maxScore {
					t.Errorf("Score = %v, want between %v and %v", result.Score, tt.minScore, tt.maxScore)
				}
			}
		})
	}
}

func TestCodeforcesFormat_RankParticipants(t *testing.T) {
	f := &CodeforcesFormat{config: DefaultConfig()}
	participants := []format.ParticipantScore{
		{UserID: 1, TotalScore: 1500, TotalPenalty: 200},
		{UserID: 2, TotalScore: 2000, TotalPenalty: 300},
		{UserID: 3, TotalScore: 1500, TotalPenalty: 100},
	}

	ranks := f.RankParticipants(participants)

	if ranks[0].Score.UserID != 2 {
		t.Errorf("first place = %d, want 2", ranks[0].Score.UserID)
	}
	if ranks[1].Score.UserID != 3 {
		t.Errorf("second place = %d, want 3", ranks[1].Score.UserID)
	}
	if ranks[2].Score.UserID != 1 {
		t.Errorf("third place = %d, want 1", ranks[2].Score.UserID)
	}
}

func TestCodeforcesFormat_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  json.RawMessage
		wantErr bool
	}{
		{
			name:    "valid default",
			config:  json.RawMessage(`{"initial_scores":[500,1000],"decay_factor":250,"min_score_ratio":0.3,"wrong_submission_penalty":50}`),
			wantErr: false,
		},
		{
			name:    "negative decay",
			config:  json.RawMessage(`{"initial_scores":[500],"decay_factor":-10}`),
			wantErr: true,
		},
		{
			name:    "invalid ratio",
			config:  json.RawMessage(`{"initial_scores":[500],"decay_factor":250,"min_score_ratio":1.5}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CodeforcesFormat{config: DefaultConfig()}
			err := f.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
