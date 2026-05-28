package postgres

import (
	"context"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

// mockDB is not needed; we test via interface compliance and nil-safe returns.

func TestGetRecommendations_NilDB_ReturnsError(t *testing.T) {
	// ProblemStore with nil db should not panic, should return error.
	s := &ProblemStore{db: nil}
	_, err := s.GetRecommendations(context.Background(), "user-1", 1200)
	if err == nil {
		t.Fatal("expected error when db is nil, got nil")
	}
}

func TestDifficultyMapping(t *testing.T) {
	// Verify difficulty mapping logic in isolation via a helper.
	tests := []struct {
		rating   int
		expected string
	}{
		{0, "easy"},
		{1000, "easy"},
		{1399, "easy"},
		{1400, "medium"},
		{1600, "medium"},
		{1899, "medium"},
		{1900, "hard"},
		{2500, "hard"},
		{3000, "hard"},
	}
	for _, tt := range tests {
		got := ratingToDifficulty(tt.rating)
		if got != tt.expected {
			t.Errorf("ratingToDifficulty(%d) = %q, want %q", tt.rating, got, tt.expected)
		}
	}
}

func TestWeakTagDefaults(t *testing.T) {
	// Verify default fallback logic.
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"no tags", nil, []string{"dp", "graphs"}},
		{"one tag", []string{"math"}, []string{"math", "greedy"}},
		{"two tags", []string{"dp", "math"}, []string{"dp", "math"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyWeakTagDefaults(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.expected))
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestBuildHybridList_NoDuplicates(t *testing.T) {
	prog := []model.ProblemListItem{
		{ID: "p1"}, {ID: "p2"}, {ID: "p3"},
	}
	weak := []model.ProblemListItem{
		{ID: "p2"}, // duplicate with progression
		{ID: "p4"},
	}
	daily := &model.ProblemListItem{ID: "p5"}

	hybrid := buildHybridList(prog, weak, daily)
	seen := map[string]bool{}
	for _, p := range hybrid {
		if seen[p.ID] {
			t.Errorf("duplicate ID %q in hybrid list", p.ID)
		}
		seen[p.ID] = true
	}
	// Should have p1, p2, p4, p5 (p2 from weak skipped as dup, p3 not included because limit 2 from prog)
	if len(hybrid) > 5 {
		t.Errorf("hybrid list too large: %d", len(hybrid))
	}
}

func TestBuildHybridList_EmptyInputs(t *testing.T) {
	hybrid := buildHybridList(nil, nil, nil)
	if hybrid == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(hybrid) != 0 {
		t.Errorf("expected empty, got %d items", len(hybrid))
	}
}
