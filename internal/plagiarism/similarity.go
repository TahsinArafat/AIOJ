package plagiarism

import (
	"context"
	"sort"
)

// maxSubmissionsPerProblem bounds the pairwise comparison. The scan is O(n²) in
// accepted submissions, so a popular problem with 1000 ACs means ~500k LCS
// comparisons; 300 keeps one problem to ~45k.
const maxSubmissionsPerProblem = 300

// Source is one submission eligible for comparison. The fields mirror what the
// engine needs and nothing else, so an engine can be tested without a database.
type Source struct {
	Key       string
	ProblemID string
	UserID    string
	Code      string
}

// Pair is one above-threshold result.
type Pair struct {
	ProblemID string
	// A and B are Source keys, ordered as the engine emitted them.
	A, B string
	// Similarity is 2·LCS/(|A|+|B|), the same ratio CompareCodes reports.
	Similarity float64
	// MatchedTokens is the LCS length. It is what the report shows as
	// "matched lines".
	MatchedTokens int
}

// ScanResult is one engine run.
type ScanResult struct {
	// Pairs holds every pair at or above the threshold.
	Pairs []Pair
	// PairsTotal is every eligible pair (same problem, different author).
	// Callers should prefer CountEligiblePairs, which cannot disagree with the
	// report; this field exists so a mismatch is observable.
	PairsTotal int
	// PairsScored counts the pairs that needed a real LCS rather than being
	// rejected by the prefilter. It is the engine's own cost signal.
	PairsScored int
}

// Engine scores submissions pairwise.
//
// Implementations must be exact: they report a pair if and only if
// `2·LCS/(|a|+|b|) >= threshold`, using the same tokenization as Tokenize. A
// prefilter is allowed, but only a *sound* one — one that can never discard a
// pair above the threshold. An engine that under-reports silently stops
// catching plagiarists, which is worse than being slow.
type Engine interface {
	Name() string
	FindSimilar(ctx context.Context, sources []Source, threshold float64) (*ScanResult, error)
}

// CountEligiblePairs is the authoritative denominator for a report: every pair
// that shares a problem and has two different authors.
//
// It lives in Go rather than being taken from the engine so that a report's
// "N pairs compared" number stays correct even if an engine's grouping logic
// drifts. A mismatch is logged by Service.
func CountEligiblePairs(sources []Source) int {
	byProblem := make(map[string][]string, len(sources))
	for _, s := range sources {
		byProblem[s.ProblemID] = append(byProblem[s.ProblemID], s.UserID)
	}
	total := 0
	for _, authors := range byProblem {
		for i := 0; i < len(authors); i++ {
			for j := i + 1; j < len(authors); j++ {
				if authors[i] != authors[j] {
					total++
				}
			}
		}
	}
	return total
}

// sortPairs gives both engines the same deterministic output order, so a report
// built from either one lists its rows identically.
func sortPairs(pairs []Pair) {
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].ProblemID != pairs[j].ProblemID {
			return pairs[i].ProblemID < pairs[j].ProblemID
		}
		if pairs[i].A != pairs[j].A {
			return pairs[i].A < pairs[j].A
		}
		return pairs[i].B < pairs[j].B
	})
}

// similarity is the ratio both engines and CompareCodes must agree on. Keeping
// it in one place is what makes "the Rust engine reports the same number" a
// property of the code rather than of two copies of a formula.
func similarityOf(lcs, lenA, lenB int) float64 {
	total := lenA + lenB
	if total == 0 {
		return 0.0
	}
	return 2.0 * float64(lcs) / float64(total)
}
