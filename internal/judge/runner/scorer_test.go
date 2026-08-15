package runner

import (
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

func TestScorer_Standard_AllAC(t *testing.T) {
	prob := &model.Problem{ScoringMode: "standard"}
	results := []model.TestCaseResult{
		{CaseName: "1.in", Status: model.StatusAC, Score: 10, Time: 100, Memory: 512},
		{CaseName: "2.in", Status: model.StatusAC, Score: 10, Time: 200, Memory: 256},
		{CaseName: "3.in", Status: model.StatusAC, Score: 10, Time: 50, Memory: 128},
	}
	out := NewScorer().Aggregate(prob, results)

	if out.FinalStatus != model.StatusAC {
		t.Errorf("expected AC, got %s", out.FinalStatus)
	}
	if out.Score != 10 {
		t.Errorf("expected avg score 10, got %d", out.Score)
	}
	if out.MaxTime != 200 {
		t.Errorf("expected maxTime 200, got %d", out.MaxTime)
	}
	if out.MaxMem != 512 {
		t.Errorf("expected maxMem 512, got %d", out.MaxMem)
	}
}

func TestScorer_Standard_FirstFailureWins(t *testing.T) {
	prob := &model.Problem{ScoringMode: "standard"}
	results := []model.TestCaseResult{
		{CaseName: "1.in", Status: model.StatusAC, Score: 10},
		{CaseName: "2.in", Status: model.StatusWA, Score: 0},
		{CaseName: "3.in", Status: model.StatusTLE, Score: 0},
	}
	out := NewScorer().Aggregate(prob, results)

	if out.FinalStatus != model.StatusWA {
		t.Errorf("expected WA (first failure), got %s", out.FinalStatus)
	}
}

func TestScorer_Subtask_MinAggregation_OneFails(t *testing.T) {
	prob := &model.Problem{
		ScoringMode:        "partial",
		SubtaskAggregation: "min",
		TestCaseScore: []model.TestCaseScore{
			{InputName: "1.in", Score: 10, SubtaskID: 1},
			{InputName: "2.in", Score: 10, SubtaskID: 1},
			{InputName: "3.in", Score: 20, SubtaskID: 2},
		},
	}
	results := []model.TestCaseResult{
		{CaseName: "1.in", Status: model.StatusAC, Score: 10},
		{CaseName: "2.in", Status: model.StatusWA, Score: 0}, // subtask 1 fails
		{CaseName: "3.in", Status: model.StatusAC, Score: 20},
	}
	out := NewScorer().Aggregate(prob, results)

	// Subtask 1 score = 0 (min aggregation); subtask 2 = 20; total 20/40 = 50%
	if out.Score != 50 {
		t.Errorf("expected score 50%%, got %d", out.Score)
	}
	if out.FinalStatus != model.StatusWA {
		t.Errorf("expected WA, got %s", out.FinalStatus)
	}
}

func TestScorer_Subtask_MinAggregation_AllPass(t *testing.T) {
	prob := &model.Problem{
		ScoringMode:        "partial",
		SubtaskAggregation: "min",
		TestCaseScore: []model.TestCaseScore{
			{InputName: "1.in", Score: 20, SubtaskID: 1},
			{InputName: "2.in", Score: 20, SubtaskID: 1},
			{InputName: "3.in", Score: 60, SubtaskID: 2},
		},
	}
	results := []model.TestCaseResult{
		{CaseName: "1.in", Status: model.StatusAC, Score: 20},
		{CaseName: "2.in", Status: model.StatusAC, Score: 20},
		{CaseName: "3.in", Status: model.StatusAC, Score: 60},
	}
	out := NewScorer().Aggregate(prob, results)

	if out.Score != 100 {
		t.Errorf("expected 100%%, got %d", out.Score)
	}
	if out.FinalStatus != model.StatusAC {
		t.Errorf("expected AC, got %s", out.FinalStatus)
	}
}

func TestScorer_Subtask_SumAggregation_PartialPass(t *testing.T) {
	prob := &model.Problem{
		ScoringMode:        "partial",
		SubtaskAggregation: "sum",
		TestCaseScore: []model.TestCaseScore{
			{InputName: "1.in", Score: 10, SubtaskID: 1},
			{InputName: "2.in", Score: 30, SubtaskID: 1},
			{InputName: "3.in", Score: 60, SubtaskID: 2},
		},
	}
	results := []model.TestCaseResult{
		{CaseName: "1.in", Status: model.StatusAC, Score: 10},
		{CaseName: "2.in", Status: model.StatusWA, Score: 0}, // sum: partial credit
		{CaseName: "3.in", Status: model.StatusAC, Score: 60},
	}
	out := NewScorer().Aggregate(prob, results)

	// subtask 1: 10 (sum, only case 1 passes); subtask 2: 60 → total 70/100 = 70%
	if out.Score != 70 {
		t.Errorf("expected 70%%, got %d", out.Score)
	}
}
