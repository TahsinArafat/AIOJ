package atcoder

import (
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/contest/format"
)

func TestAtCoderFormat_Name(t *testing.T) {
	f := &AtCoderFormat{config: DefaultConfig()}
	if f.Name() != "atcoder" {
		t.Errorf("Name() = %v, want %v", f.Name(), "atcoder")
	}
}

func TestAtCoderFormat_ScoreProblem(t *testing.T) {
	start := time.Now()
	tests := []struct {
		name        string
		submissions []format.Submission
		wantSolved  bool
		wantPenalty int
	}{
		{
			name:        "no submissions",
			submissions: nil,
			wantSolved:  false,
			wantPenalty: 0,
		},
		{
			name: "AC at 30 minutes with wrong attempts",
			submissions: []format.Submission{
				{Status: "WA", CreatedAt: start.Add(10 * time.Minute)},
				{Status: "WA", CreatedAt: start.Add(20 * time.Minute)},
				{Status: "AC", CreatedAt: start.Add(30 * time.Minute)},
			},
			wantSolved:  true,
			wantPenalty: 30, // No penalty for wrong attempts
		},
		{
			name: "first try AC",
			submissions: []format.Submission{
				{Status: "AC", CreatedAt: start.Add(45 * time.Minute)},
			},
			wantSolved:  true,
			wantPenalty: 45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &AtCoderFormat{config: DefaultConfig()}
			ctx := format.ScoringContext{
				SubmissionStartTime: start,
				Problem:             format.Problem{ID: 1, Index: "A"},
				Submissions:         tt.submissions,
			}

			result, err := f.ScoreProblem(ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Solved != tt.wantSolved {
				t.Errorf("Solved = %v, want %v", result.Solved, tt.wantSolved)
			}
			if result.Penalty != tt.wantPenalty {
				t.Errorf("Penalty = %v, want %v", result.Penalty, tt.wantPenalty)
			}
		})
	}
}
