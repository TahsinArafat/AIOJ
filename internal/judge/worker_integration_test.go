package judge

import (
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

func TestGetSubtasks_TwoGroups(t *testing.T) {
	prob := &model.Problem{
		ScoringMode:        "partial",
		SubtaskAggregation: "min",
		TestCaseScore: []model.TestCaseScore{
			{InputName: "1.in", OutputName: "1.out", Score: 10, SubtaskID: 1},
			{InputName: "2.in", OutputName: "2.out", Score: 10, SubtaskID: 1},
			{InputName: "3.in", OutputName: "3.out", Score: 20, SubtaskID: 2},
		},
	}

	subtasks := prob.GetSubtasks()
	if len(subtasks) != 2 {
		t.Fatalf("expected 2 subtasks, got %d", len(subtasks))
	}
	if len(subtasks[1]) != 2 {
		t.Errorf("subtask 1: expected 2 cases, got %d", len(subtasks[1]))
	}
	if len(subtasks[2]) != 1 {
		t.Errorf("subtask 2: expected 1 case, got %d", len(subtasks[2]))
	}
}

func TestGetSubtasks_Empty(t *testing.T) {
	prob := &model.Problem{
		TestCaseScore: []model.TestCaseScore{
			{InputName: "1.in", OutputName: "1.out", Score: 10},
		},
	}
	if prob.HasSubtasks() {
		t.Error("expected HasSubtasks() = false for cases without SubtaskID")
	}
	if len(prob.GetSubtasks()) != 0 {
		t.Error("expected empty subtask map")
	}
}

func TestSubtaskScoreAggregation_MinAllPass(t *testing.T) {
	// All 3 test cases in subtask 1 pass — score should be 30
	cases := []model.TestCaseScore{
		{InputName: "1.in", OutputName: "1.out", Score: 10, SubtaskID: 1},
		{InputName: "2.in", OutputName: "2.out", Score: 10, SubtaskID: 1},
		{InputName: "3.in", OutputName: "3.out", Score: 10, SubtaskID: 1},
	}

	subtaskScore := computeSubtaskScore(cases, []bool{true, true, true}, "min")
	if subtaskScore != 30 {
		t.Errorf("expected score 30, got %d", subtaskScore)
	}
}

func TestSubtaskScoreAggregation_MinOneFails(t *testing.T) {
	// 2 pass, 1 fails — min aggregation zeroes the whole subtask
	cases := []model.TestCaseScore{
		{InputName: "1.in", OutputName: "1.out", Score: 10, SubtaskID: 1},
		{InputName: "2.in", OutputName: "2.out", Score: 10, SubtaskID: 1},
		{InputName: "3.in", OutputName: "3.out", Score: 10, SubtaskID: 1},
	}

	subtaskScore := computeSubtaskScore(cases, []bool{true, false, true}, "min")
	if subtaskScore != 0 {
		t.Errorf("expected score 0 (min aggregation, one failure), got %d", subtaskScore)
	}
}

func TestSubtaskScoreAggregation_SumPartialPass(t *testing.T) {
	// 2 pass, 1 fails — sum aggregation gives partial credit
	cases := []model.TestCaseScore{
		{InputName: "1.in", OutputName: "1.out", Score: 10, SubtaskID: 1},
		{InputName: "2.in", OutputName: "2.out", Score: 15, SubtaskID: 1},
		{InputName: "3.in", OutputName: "3.out", Score: 25, SubtaskID: 1},
	}

	// Cases 1 and 3 pass (10 + 25 = 35), case 2 fails
	subtaskScore := computeSubtaskScore(cases, []bool{true, false, true}, "sum")
	if subtaskScore != 35 {
		t.Errorf("expected score 35 (sum aggregation), got %d", subtaskScore)
	}
}

// computeSubtaskScore is a test helper that simulates the aggregation logic
// from evaluateSubtasks — does NOT depend on the judge worker internals.
func computeSubtaskScore(cases []model.TestCaseScore, passed []bool, aggregation string) int {
	score := 0
	failed := false
	for i, tc := range cases {
		if passed[i] {
			if !failed {
				score += tc.Score
			}
		} else {
			if aggregation == "min" {
				failed = true
				score = 0
			}
		}
	}
	return score
}
