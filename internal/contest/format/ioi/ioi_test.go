package ioi

import (
	"encoding/json"
	"testing"

	"github.com/tahsinarafat/aioj/internal/contest/format"
)

func TestIOIFormat_Name(t *testing.T) {
	f := &IOIFormat{config: DefaultConfig()}
	if f.Name() != "ioi" {
		t.Errorf("Name() = %v, want %v", f.Name(), "ioi")
	}
}

func TestIOIFormat_ScoreProblem_NoSubtasks(t *testing.T) {
	f := &IOIFormat{config: DefaultConfig()}
	ctx := format.ScoringContext{
		Problem: format.Problem{ID: "1", Index: "A"},
		Submissions: []format.Submission{
			{Score: 80},
			{Score: 95},
		},
	}

	result, err := f.ScoreProblem(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Score != 95 {
		t.Errorf("Score = %v, want %v", result.Score, 95)
	}
	if result.Solved {
		t.Errorf("Solved = %v, want false", result.Solved)
	}
}

func TestIOIFormat_ScoreProblem_WithSubtasks(t *testing.T) {
	f := &IOIFormat{config: DefaultConfig()}
	problemCfg := ProblemConfig{
		Subtasks: []SubtaskConfig{
			{ID: "st1", Points: 30, TestCases: 5},
			{ID: "st2", Points: 70, TestCases: 10},
		},
	}
	cfgJSON, _ := json.Marshal(problemCfg)

	ctx := format.ScoringContext{
		Problem: format.Problem{
			ID:     "1",
			Index:  "A",
			Config: cfgJSON,
		},
		Submissions: []format.Submission{
			{Score: 100},
		},
	}

	result, err := f.ScoreProblem(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SubtaskScores["st1"] != 30 {
		t.Errorf("st1 score = %v, want %v", result.SubtaskScores["st1"], 30)
	}
	if result.SubtaskScores["st2"] != 70 {
		t.Errorf("st2 score = %v, want %v", result.SubtaskScores["st2"], 70)
	}
	if result.Score != 100 {
		t.Errorf("Score = %v, want %v", result.Score, 100)
	}
}
