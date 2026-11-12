package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// discoverCaseScores is what the AI apply path runs over a promoted testdata
// directory. It has to total exactly 100 points however many pairs there are,
// because the judge's score is a percentage of the points available.
func TestDiscoverCaseScoresSpreads100Points(t *testing.T) {
	for _, n := range []int{1, 3, 12} {
		dir := t.TempDir()
		for i := 0; i < n; i++ {
			stem := fmt.Sprintf("case%02d", i)
			if err := os.WriteFile(filepath.Join(dir, stem+".in"), []byte("1"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, stem+".out"), []byte("2"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		cases := discoverCaseScores(dir)
		if len(cases) != n {
			t.Fatalf("%d pairs: discovered %d", n, len(cases))
		}
		total := 0
		for _, c := range cases {
			total += c.Score
			if c.OutputName != strings.TrimSuffix(c.InputName, ".in")+".out" {
				t.Errorf("case %s paired with %s", c.InputName, c.OutputName)
			}
		}
		if total != 100 {
			t.Errorf("%d pairs total %d points, want 100", n, total)
		}
	}
}

// An input without a matching output is not a test case and must be skipped
// rather than scored.
func TestDiscoverCaseScoresSkipsUnpairedInputs(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.in"), []byte("1"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "a.out"), []byte("2"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "orphan.in"), []byte("1"), 0o644)

	cases := discoverCaseScores(dir)
	if len(cases) != 1 || cases[0].InputName != "a.in" {
		t.Fatalf("cases = %+v, want just a.in", cases)
	}
}
