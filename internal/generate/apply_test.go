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

// problems.memory_limit is kilobytes. A proposal's memory_limit_kb must land in
// the column unchanged; multiplying by 1024 here stored bytes and made the
// problem page show "262144000 MB".
func TestProblemMemoryLimitIsStoredAsKilobytes(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"explicit 256 MB", 262144, 262144},
		{"explicit 512 MB", 524288, 524288},
		{"explicit non-round", 256000, 256000},
		{"omitted falls back to 256 MB", 0, 262144},
		{"negative falls back to 256 MB", -1, 262144},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := problemMemoryLimitKB(Proposal{MemoryLimitKB: tc.in})
			if got != tc.want {
				t.Fatalf("problemMemoryLimitKB(%d) = %d, want %d KB", tc.in, got, tc.want)
			}
		})
	}
}
