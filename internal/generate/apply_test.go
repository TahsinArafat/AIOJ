package generate

import (
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

func TestAssignScoresTotalsExactlyOneHundred(t *testing.T) {
	// A flat 10 per case meant a 12-case problem was scored out of 120.
	for _, n := range []int{1, 2, 3, 7, 12, 13, 100} {
		cases := make([]model.TestCaseScore, n)
		assignScores(cases)

		total := 0
		for _, c := range cases {
			total += c.Score
			if c.Score <= 0 {
				t.Fatalf("%d cases: a case got %d points", n, c.Score)
			}
		}
		if total != 100 {
			t.Fatalf("%d cases total %d points, want 100", n, total)
		}
	}
}

func TestAssignScoresIsEmptySafe(t *testing.T) {
	assignScores(nil) // must not panic
}

func TestDefaultIntFallsBackOnlyForNonPositive(t *testing.T) {
	if got := defaultInt(0, 2000); got != 2000 {
		t.Fatalf("got %d, want the fallback", got)
	}
	if got := defaultInt(-5, 2000); got != 2000 {
		t.Fatalf("got %d, want the fallback", got)
	}
	if got := defaultInt(1500, 2000); got != 1500 {
		t.Fatalf("got %d, want the explicit value", got)
	}
}
