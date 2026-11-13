package format_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/contest/format"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/acm"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/atcoder"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/codeforces"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/ioi"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/oi"
)

// A skipped submission is one an organizer voided: the round must be scored
// exactly as if it had never been made. That is stronger than "it scores
// nothing" — it may not count as an attempt, and it may not contribute to the
// wrong-attempt penalty on the submission that was accepted.
//
// The formats receive UPPER-CASE statuses, because the scoreboard handler
// upper-cases the stored enum before scoring. So the voided status arrives here
// as "SKIPPED", and these cases are written the way the handler actually calls.
//
// Every format that counts attempts is covered. ioi and oi are included and
// asserted only on the score/solve outcome, because they never count attempts at
// all — a skipped row is invisible to them through the filter regardless.
func TestSkippedIsNotAnAttempt(t *testing.T) {
	start := time.Now()

	// The shape being protected: two wrong answers, then a skip, then an
	// accepted run. If the skip counted, the penalty would be 2 x 20 instead of
	// 1 x 20, and Attempts would be 4 instead of 3.
	voided := func() []format.Submission {
		return []format.Submission{
			{Status: "WA", Score: 0, CreatedAt: start.Add(10 * time.Minute)},
			{Status: "SKIPPED", Score: 0, CreatedAt: start.Add(20 * time.Minute)},
			{Status: "AC", Score: 100, CreatedAt: start.Add(60 * time.Minute)},
		}
	}

	t.Run("acm: the voided row leaves no penalty and no attempt", func(t *testing.T) {
		f, err := format.Create("acm", nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := f.ScoreProblem(format.ScoringContext{
			SubmissionStartTime: start,
			ContestDuration:     2 * time.Hour,
			Problem:             format.Problem{ID: "p", Index: "A"},
			Submissions:         voided(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if !res.Solved {
			t.Error("the accepted run must still solve the problem")
		}
		if res.Attempts != 2 {
			t.Errorf("Attempts = %d, want 2 — a skipped submission is not an attempt", res.Attempts)
		}
		// 60 minutes + one wrong (20).
		if res.Penalty != 80 {
			t.Errorf("Penalty = %d, want 80 (60 + 1x20) — the skipped row must not add a wrong-attempt penalty", res.Penalty)
		}
	})

	t.Run("codeforces: the voided row leaves no score decay", func(t *testing.T) {
		f, err := format.Create("codeforces", nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := f.ScoreProblem(format.ScoringContext{
			SubmissionStartTime: start,
			ContestDuration:     2 * time.Hour,
			Problem:             format.Problem{ID: "p", Index: "A"},
			Submissions:         voided(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if !res.Solved {
			t.Error("the accepted run must still solve the problem")
		}
		if res.Attempts != 2 {
			t.Errorf("Attempts = %d, want 2 — a skipped submission is not an attempt", res.Attempts)
		}
		// Wrong-submission penalty is 50 per wrong before the AC, so 1 x 50
		// rather than 2 x 50.
		if res.Penalty != 60+20 {
			t.Errorf("Penalty = %d, want 80 (60 minutes + 1x20) — the skipped row must not count as wrong", res.Penalty)
		}
	})

	t.Run("atcoder: the voided row is not counted", func(t *testing.T) {
		f, err := format.Create("atcoder", nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := f.ScoreProblem(format.ScoringContext{
			SubmissionStartTime: start,
			ContestDuration:     2 * time.Hour,
			Problem:             format.Problem{ID: "p", Index: "A"},
			Submissions:         voided(),
		})
		if err != nil {
			t.Fatal(err)
		}
		if !res.Solved {
			t.Error("the accepted run must still solve the problem")
		}
		if res.Attempts != 2 {
			t.Errorf("Attempts = %d, want 2 — a skipped submission is not an attempt", res.Attempts)
		}
	})

	// A void is what makes "every submission was skipped" possible, and that
	// must score as a person who never submitted — not as a crash, and not as a
	// wrong attempt.
	t.Run("all submissions skipped scores as if the person never submitted", func(t *testing.T) {
		allSkipped := []format.Submission{
			{Status: "SKIPPED", Score: 0, CreatedAt: start.Add(10 * time.Minute)},
			{Status: "SKIPPED", Score: 0, CreatedAt: start.Add(20 * time.Minute)},
		}
		for _, name := range []string{"acm", "codeforces", "atcoder"} {
			f, err := format.Create(name, nil)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			res, err := f.ScoreProblem(format.ScoringContext{
				SubmissionStartTime: start,
				ContestDuration:     2 * time.Hour,
				Problem:             format.Problem{ID: "p", Index: "A"},
				Submissions:         allSkipped,
			})
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if res.Solved || res.Score != 0 || res.Attempts != 0 || res.Penalty != 0 {
				t.Errorf("%s: an all-skipped problem scored %+v, want the zero result", name, res)
			}
		}
	})

	// ioi/oi score by threshold (the highest score of any submission), so a void
	// must not be able to contribute points. This is the case where leaving the
	// row in would actually change the standing rather than just the penalty.
	t.Run("score-threshold formats cannot score from a voided row", func(t *testing.T) {
		withSkipped := []format.Submission{
			{Status: "SKIPPED", Score: 100, CreatedAt: start.Add(10 * time.Minute)},
		}
		for _, name := range []string{"ioi", "oi"} {
			f, err := format.Create(name, nil)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			res, err := f.ScoreProblem(format.ScoringContext{
				SubmissionStartTime: start,
				ContestDuration:     2 * time.Hour,
				Problem:             format.Problem{ID: "p", Index: "A", Config: json.RawMessage(`{}`)},
				Submissions:         withSkipped,
			})
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if res.Score != 0 || res.Solved {
				t.Errorf("%s: a skipped submission still scored %v (solved=%v); its points must not count", name, res.Score, res.Solved)
			}
		}
	})
}

// The status the handler actually stores and upper-cases must be the one the
// formats filter on. A typo here would silently disable the whole feature: the
// filter would match nothing and every skipped row would count as a wrong
// attempt again.
func TestSkippedStatusMatchesTheScoreboardSpelling(t *testing.T) {
	if format.StatusSkipped != "SKIPPED" {
		t.Fatalf("format.StatusSkipped = %q, want %q — it must be the upper-case form of the stored status, which is what the scoreboard passes in", format.StatusSkipped, "SKIPPED")
	}
	if !format.IsSkipped("SKIPPED") || !format.IsSkipped("skipped") {
		t.Error("IsSkipped must accept the status in either case, so the two spellings cannot drift apart")
	}
	if format.IsSkipped("AC") || format.IsSkipped("") {
		t.Error("IsSkipped must not match a real verdict")
	}
}
