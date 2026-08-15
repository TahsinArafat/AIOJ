package runner

import (
	"sort"

	"github.com/tahsinarafat/aioj/internal/model"
)

// Scorer aggregates a flat slice of per-test-case results into a final
// verdict, score, and resource-usage summary.
//
// It supports three scoring modes:
//   - standard:  first-failure wins; average per-test score.
//   - partial:   subtask aggregation (min or sum).
//   - (default): treated as standard.
type Scorer struct{}

// NewScorer returns a Scorer.
func NewScorer() *Scorer { return &Scorer{} }

// Aggregate computes the final RunOutput from raw per-test results.
// prob is needed for subtask definitions and aggregation mode.
func (s *Scorer) Aggregate(prob *model.Problem, results []model.TestCaseResult) RunOutput {
	if prob.HasSubtasks() && prob.ScoringMode == "partial" {
		return s.aggregateSubtasks(prob, results)
	}
	return s.aggregateStandard(results)
}

// aggregateStandard: first non-AC status wins; score = avg per-test score.
func (s *Scorer) aggregateStandard(results []model.TestCaseResult) RunOutput {
	finalStatus := model.StatusAC
	totalScore, maxTime, maxMem := 0, 0, 0

	for _, r := range results {
		if r.Status != model.StatusAC && finalStatus == model.StatusAC {
			finalStatus = r.Status
		}
		totalScore += r.Score
		if r.Time > maxTime {
			maxTime = r.Time
		}
		if r.Memory > maxMem {
			maxMem = r.Memory
		}
	}

	score := 0
	if len(results) > 0 {
		score = totalScore / len(results)
	}

	return RunOutput{
		Results:     results,
		FinalStatus: finalStatus,
		Score:       score,
		MaxTime:     maxTime,
		MaxMem:      maxMem,
	}
}

// aggregateSubtasks: groups results back into subtasks and applies
// min/sum aggregation within each group.
func (s *Scorer) aggregateSubtasks(prob *model.Problem, results []model.TestCaseResult) RunOutput {
	// Build a name→result lookup so we can map subtask test cases.
	byName := make(map[string]model.TestCaseResult, len(results))
	for _, r := range results {
		byName[r.CaseName] = r
	}

	subtasks := prob.GetSubtasks()
	var subtaskIDs []int
	for id := range subtasks {
		subtaskIDs = append(subtaskIDs, id)
	}
	sort.Ints(subtaskIDs)

	finalStatus := model.StatusAC
	totalScore, maxScore := 0, 0
	maxTime, maxMem := 0, 0

	for _, stID := range subtaskIDs {
		cases := subtasks[stID]
		subtaskScore := 0
		subtaskFailed := false

		for _, tc := range cases {
			maxScore += tc.Score
			r, ok := byName[tc.InputName]
			if !ok {
				// Shouldn't happen, but treat missing as SE.
				subtaskFailed = true
				subtaskScore = 0
				if finalStatus == model.StatusAC {
					finalStatus = model.StatusSE
				}
				continue
			}

			if r.Time > maxTime {
				maxTime = r.Time
			}
			if r.Memory > maxMem {
				maxMem = r.Memory
			}

			if r.Status == model.StatusAC {
				if !subtaskFailed {
					subtaskScore += tc.Score
				}
			} else {
				if prob.SubtaskAggregation == "min" {
					subtaskFailed = true
					subtaskScore = 0
				}
				if finalStatus == model.StatusAC {
					finalStatus = r.Status
				}
			}
		}
		totalScore += subtaskScore
	}

	percentageScore := 0
	if maxScore > 0 {
		percentageScore = (totalScore * 100) / maxScore
	}

	return RunOutput{
		Results:     results,
		FinalStatus: finalStatus,
		Score:       percentageScore,
		MaxTime:     maxTime,
		MaxMem:      maxMem,
	}
}
