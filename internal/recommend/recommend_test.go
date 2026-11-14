package recommend_test

import (
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/recommend"
)

/**
 * The recommendation policy.
 *
 * These rules lived in internal/store/postgres/recommendations.go, reachable only
 * through a database query. As pure functions of their inputs they can be
 * asserted directly, which is what this file does.
 */

func TestDifficultyFor(t *testing.T) {
	tests := []struct {
		rating int
		want   string
	}{
		{0, "easy"},
		{1000, "easy"},
		{1399, "easy"},
		// The boundaries are the policy: 1400 and 1900.
		{1400, "medium"},
		{1600, "medium"},
		{1899, "medium"},
		{1900, "hard"},
		{2500, "hard"},
		{3000, "hard"},
	}
	for _, tt := range tests {
		if got := recommend.DifficultyFor(tt.rating); got != tt.want {
			t.Errorf("DifficultyFor(%d) = %q, want %q", tt.rating, got, tt.want)
		}
	}
}

func TestDefaultWeakTags(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"no tags", nil, []string{"dp", "graphs"}},
		{"one tag", []string{"math"}, []string{"math", "greedy"}},
		{"two tags kept", []string{"dp", "math"}, []string{"dp", "math"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recommend.DefaultWeakTags(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// The one-tag case must not discard the tag it found, and must not alias the
// caller's slice in a way that lets a later append overwrite it.
func TestDefaultWeakTagsKeepsFoundTag(t *testing.T) {
	found := []string{"math"}
	got := recommend.DefaultWeakTags(found)
	if got[0] != "math" {
		t.Errorf("the found tag was dropped: %v", got)
	}
	if got[1] != "greedy" {
		t.Errorf("want a second tag, got %v", got)
	}
}

func TestHybridDeduplicates(t *testing.T) {
	prog := []model.ProblemListItem{{ID: "p1"}, {ID: "p2"}, {ID: "p3"}}
	// p2 also appears in progression; a hybrid that shows it twice is a bug the
	// user sees even though each source is correct.
	weak := []model.ProblemListItem{{ID: "p2"}, {ID: "p4"}}
	daily := &model.ProblemListItem{ID: "p5"}

	hybrid := recommend.Hybrid(prog, weak, daily)

	seen := map[string]bool{}
	for _, p := range hybrid {
		if seen[p.ID] {
			t.Errorf("duplicate id %q in the hybrid list", p.ID)
		}
		seen[p.ID] = true
	}
	// Two from progression (p1, p2), then p4 (p2 skipped as a duplicate), then p5.
	want := []string{"p1", "p2", "p4", "p5"}
	if len(hybrid) != len(want) {
		t.Fatalf("hybrid = %v, want %v", hybrid, want)
	}
	for i, id := range want {
		if hybrid[i].ID != id {
			t.Errorf("hybrid[%d] = %q, want %q", i, hybrid[i].ID, id)
		}
	}
}

func TestHybridEmptyInputs(t *testing.T) {
	hybrid := recommend.Hybrid(nil, nil, nil)
	if hybrid == nil {
		t.Fatal("an empty recommendation list must be an empty slice, not nil")
	}
	if len(hybrid) != 0 {
		t.Errorf("want an empty list, got %v", hybrid)
	}
}

// An empty candidate set must yield no daily challenge, not a panic.
func TestPickDailyNoCandidates(t *testing.T) {
	if got := recommend.PickDaily(nil, nil); got != nil {
		t.Errorf("PickDaily(nil) = %+v, want nil", got)
	}
}

// The injected picker decides which candidate wins, so the choice is testable.
func TestPickDailyUsesInjectedPicker(t *testing.T) {
	candidates := []model.ProblemListItem{{ID: "a"}, {ID: "b"}, {ID: "c"}}

	var seenN int
	got := recommend.PickDaily(candidates, func(n int) int {
		seenN = n
		return 2
	})
	if seenN != 3 {
		t.Errorf("picker was offered %d candidates, want 3", seenN)
	}
	if got == nil || got.ID != "c" {
		t.Errorf("PickDaily = %+v, want the third candidate", got)
	}
}

// Build wires the policy together. This is the function the store now calls
// after running its queries.
func TestBuild(t *testing.T) {
	prog := []model.ProblemListItem{{ID: "p1"}, {ID: "p2"}}
	weak := []model.ProblemListItem{{ID: "p3"}}
	daily := []model.ProblemListItem{{ID: "p9"}}

	resp := recommend.Build(1200, prog, weak, nil, daily, func(int) int { return 0 })

	if resp.WeakTags.Tags == nil || len(resp.WeakTags.Tags) != 2 {
		t.Errorf("weak tags = %v, want the two defaults for a user with no failed submissions", resp.WeakTags.Tags)
	}
	if len(resp.Progression) != 2 {
		t.Errorf("progression = %v, want the two rows passed in", resp.Progression)
	}
	// The hybrid draws from every source.
	if len(resp.Hybrid) != 4 {
		t.Errorf("hybrid has %d entries, want 4 (two progression, one weak, one daily): %v",
			len(resp.Hybrid), resp.Hybrid)
	}
}

// A user with no failed submissions has no weak tags from the query, but the
// recommendation must still name topics to work on.
//
// The store applies DefaultWeakTags before it searches for weak-tag problems,
// because that search is driven by the tag list. A version that searched with the
// raw list and defaulted only afterwards returned no weak-tag problems at all for
// exactly the users who most need a recommendation — which is what the store's
// integration test caught.
func TestDefaultedTagsDriveTheWeakTagSearch(t *testing.T) {
	raw := []string(nil)
	defaulted := recommend.DefaultWeakTags(raw)

	if len(defaulted) == 0 {
		t.Fatal("a user with no failed submissions still needs weak tags to search with")
	}
	// The store's query uses these to find problems, so they must be non-empty.
	if len(recommend.DefaultWeakTags(defaulted)) != len(defaulted) {
		t.Error("DefaultWeakTags is not idempotent, so applying it twice changes the search set")
	}
}
