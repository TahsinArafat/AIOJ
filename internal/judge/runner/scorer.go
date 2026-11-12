package runner

import (
	"sort"

	"github.com/tahsinarafat/aioj/internal/model"
)

// Scorer aggregates a flat slice of per-test-case results into a final
// verdict, score, and resource-usage summary.
//
// Scoring modes:
//   - standard:  first-failure wins; ratio of points earned to points available.
//   - partial:   subtask aggregation (min or sum); same ratio.
//   - (default): treated as standard.
//
// Score is always a 0-100 percentage. Every consumer of this number expects that
// scale: the contest formats multiply it back up (ioi.go:132, oi.go:65), and the
// problem page renders it as a percentage. It used to be a per-case mean, which
// only coincidentally looked like a percentage when a problem's cases summed to
// exactly 100 — every other layout silently scaled wrong.
type Scorer struct{}

// NewScorer returns a Scorer.
func NewScorer() *Scorer { return &Scorer{} }

// Aggregate computes the final RunOutput from raw per-test results.
// prob is needed for subtask definitions and aggregation mode.
func (s *Scorer) Aggregate(prob *model.Problem, results []model.TestCaseResult) RunOutput {
	if prob.HasSubtasks() && prob.ScoringMode == "partial" {
		return s.aggregateSubtasks(prob, results)
	}
	// The runner executes cases in TestCaseScore order, so the weights line up
	// index-for-index with the results it produced.
	weights := make([]int, len(prob.TestCaseScore))
	for i, tc := range prob.TestCaseScore {
		weights[i] = tc.Score
	}
	return s.aggregateStandard(weights, results)
}

// aggregateStandard: first non-AC status wins; score = points earned as a
// percentage of points available.
//
// Each case carries its own weight in TestCaseScore, so the denominator is the
// sum of those weights, not a flat per-case count. Every result the runner
// produced counts toward that denominator, so a case that reported SE still
// contributes its weight — a submission is not credited for a case the judge
// failed to run.
//
// An unweighted test set (every case worth 0) falls back to counting cases, one
// point each. That is the only sensible reading of "how much of this problem did
// you solve", and it keeps a problem created before scores were assigned from
// reporting 0% on a full solve.
func (s *Scorer) aggregateStandard(weights []int, results []model.TestCaseResult) RunOutput {
	finalStatus := model.StatusAC
	totalScore, available, maxTime, maxMem := 0, 0, 0, 0

	for i, r := range results {
		if r.Status != model.StatusAC && finalStatus == model.StatusAC {
			finalStatus = r.Status
		}
		totalScore += r.Score
		available += weightAt(weights, i)
		if r.Time > maxTime {
			maxTime = r.Time
		}
		if r.Memory > maxMem {
			maxMem = r.Memory
		}
	}

	if available <= 0 {
		// Unweighted: one point per case, and credit only for AC.
		totalScore = 0
		for _, r := range results {
			if r.Status == model.StatusAC {
				totalScore += 1
			}
		}
		available = len(results)
	}

	return RunOutput{
		Results:     results,
		FinalStatus: finalStatus,
		Score:       percentage(totalScore, available),
		MaxTime:     maxTime,
		MaxMem:      maxMem,
	}
}

// weightAt reads a case weight without panicking on a short slice, so a caller
// that hands over fewer weights than results degrades to the unweighted fallback
// instead of mis-indexing.
func weightAt(weights []int, i int) int {
	if i < 0 || i >= len(weights) {
		return 0
	}
	return weights[i]
}

// percentage turns earned/available points into a 0-100 score, rounded to
// nearest. Rounding is the reason a submission that earned a single point out of
// a thousand still reports 0 rather than being inflated; that is correct for a
// percentage display, and the raw case breakdown shows the detail.
func percentage(earned, available int) int {
	if available <= 0 {
		return 0
	}
	return (earned*100 + available/2) / available
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

	percentageScore := percentage(totalScore, maxScore)

	return RunOutput{
		Results:     results,
		FinalStatus: finalStatus,
		Score:       percentageScore,
		MaxTime:     maxTime,
		MaxMem:      maxMem,
	}
}
