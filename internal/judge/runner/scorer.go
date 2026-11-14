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
	weights := make([]int, len(prob.TestCaseScore))
	for i, tc := range prob.TestCaseScore {
		weights[i] = tc.Score
	}
	return s.aggregateStandard(weights, len(prob.TestCaseScore), results)
}

// aggregateStandard: first non-AC status wins; score = points earned as a
// percentage of points available.
//
// Each case carries its own weight in TestCaseScore, so the denominator is the
// sum of those weights, not a flat per-case count. A case that reported SE still
// contributes its weight — a submission is not credited for a case the judge
// failed to run.
//
// The denominator is the whole problem, not just the cases that ran. A fail-fast
// run stops at the first rejection, so results is a prefix of the problem's
// cases; dividing by that prefix would award a submission that failed late a
// higher score than one that failed early on the same number of passes, and
// would make "stopped at case 3" report 100% of the points it never attempted.
//
// An unweighted test set (every case worth 0) falls back to counting cases, one
// point each. That is the only sensible reading of "how much of this problem did
// you solve", and it keeps a problem created before scores were assigned from
// reporting 0% on a full solve.
func (s *Scorer) aggregateStandard(weights []int, totalCases int, results []model.TestCaseResult) RunOutput {
	finalStatus := model.StatusAC
	totalScore, available, maxTime, maxMem := 0, 0, 0, 0

	for _, w := range weights {
		available += w
	}

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

	if available <= 0 {
		// Unweighted: one point per case, and credit only for AC. A caller that
		// hands over more results than the problem declares still counts them,
		// so a synthetic problem with no weights is not scored as 0 of 0.
		totalScore = 0
		for _, r := range results {
			if r.Status == model.StatusAC {
				totalScore += 1
			}
		}
		available = max(totalCases, len(results))
	}

	return RunOutput{
		Results:     results,
		FinalStatus: finalStatus,
		Score:       percentage(totalScore, available),
		MaxTime:     maxTime,
		MaxMem:      maxMem,
	}
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
//
// A case with SubtaskID == 0 belongs to no subtask but is still scored: it forms
// its own bucket, worth exactly its own score, so it neither inflates nor
// vanishes from the total. GetSubtasks omits those cases, so relying on it alone
// dropped them from the denominator and let a submission that failed them still
// report 100%.
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

	// Cases outside every subtask are their own bucket: worth exactly their own
	// points, so they count in both the numerator and the denominator rather
	// than disappearing from the total.
	for _, tc := range prob.TestCaseScore {
		if tc.SubtaskID > 0 {
			continue
		}
		maxScore += tc.Score
		r, ok := byName[tc.InputName]
		if !ok {
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
			totalScore += tc.Score
		} else if finalStatus == model.StatusAC {
			finalStatus = r.Status
		}
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
