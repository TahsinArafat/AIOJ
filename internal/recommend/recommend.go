// Package recommend holds the recommendation policy: which problems to suggest,
// and how to combine them.
//
// The queries that feed it live in the store. This package decides what the rows
// mean, which is the part worth testing — and it was previously untestable
// without a database, because the rules sat inline in a postgres method.
package recommend

import (
	"math/rand"

	"github.com/tahsinarafat/aioj/internal/model"
)

// DifficultyFor maps a contest rating to a problem difficulty band.
//
// The bands are the recommendation policy's central assumption: a user is
// expected to be able to solve problems in their own band, and problems outside
// it are noise. The boundaries are rated-round conventions (1400 and 1900) rather
// than anything derived from this platform's data, which is a known soft spot —
// see RecommendationsResponse.
func DifficultyFor(rating int) string {
	if rating < 1400 {
		return "easy"
	}
	if rating < 1900 {
		return "medium"
	}
	return "hard"
}

// DefaultWeakTags fills in topics to work on when the query found fewer than two.
//
// The recommendation is only useful if it can name something. A user with no
// failed submissions has no weak topics by definition, and a user with exactly
// one gets a second so the weak-tag list has the shape the UI renders.
//
// The specific defaults (dp, graphs, greedy) are this platform's opinion about
// what a contestant should learn next. They are a policy choice, not a
// measurement, and they belong here rather than in a SQL file.
func DefaultWeakTags(tags []string) []string {
	switch len(tags) {
	case 0:
		return []string{"dp", "graphs"}
	case 1:
		return append(tags, "greedy")
	default:
		return tags
	}
}

// PickDaily selects one problem for the daily challenge.
//
// Random over the caller's candidates, so the challenge changes between requests
// rather than being whatever the query happened to return first. The randomness
// is injected rather than read from a global, which is what lets a test pin the
// choice.
func PickDaily(candidates []model.ProblemListItem, pick func(n int) int) *model.ProblemListItem {
	if len(candidates) == 0 {
		return nil
	}
	if pick == nil {
		pick = rand.Intn
	}
	selected := candidates[pick(len(candidates))]
	return &selected
}

// Hybrid combines up to two progression problems, up to two weak-tag problems,
// and the daily challenge into one deduplicated list.
//
// Deduplication is by problem id and is the whole reason this is a function: the
// three sources overlap by construction (a weak-tag problem is often also in the
// user's difficulty band), and a recommendation list that shows the same problem
// twice reads as a bug to the user even though each source is correct.
func Hybrid(progression, weak []model.ProblemListItem, daily *model.ProblemListItem) []model.ProblemListItem {
	seen := make(map[string]struct{})
	hybrid := make([]model.ProblemListItem, 0, 5)

	addUnique := func(item model.ProblemListItem) {
		if _, ok := seen[item.ID]; ok {
			return
		}
		seen[item.ID] = struct{}{}
		hybrid = append(hybrid, item)
	}

	for i := 0; i < len(progression) && i < 2; i++ {
		addUnique(progression[i])
	}
	for i := 0; i < len(weak) && i < 2; i++ {
		addUnique(weak[i])
	}
	if daily != nil {
		addUnique(*daily)
	}
	return hybrid
}

// Build assembles the response from rows the store fetched.
//
// Split from the queries so the policy can be tested against literal rows. The
// store method is now: run four queries, hand the results here.
func Build(
	currentRating int,
	progression, weakProblems []model.ProblemListItem,
	weakTags []string,
	dailyCandidates []model.ProblemListItem,
	pick func(n int) int,
) *model.RecommendationsResponse {
	// Idempotent on an already-defaulted set (two tags pass through unchanged), so
	// a caller that applied the defaults before querying is not double-filled.
	tags := DefaultWeakTags(weakTags)
	return &model.RecommendationsResponse{
		Progression: progression,
		WeakTags: model.WeakTagsRecommendations{
			Tags:     tags,
			Problems: weakProblems,
		},
		Hybrid: Hybrid(progression, weakProblems, PickDaily(dailyCandidates, pick)),
	}
}
