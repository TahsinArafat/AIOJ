package format_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/contest/format"
	_ "github.com/tahsinarafat/aioj/internal/contest/format/acm"
	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Standings assembly.
 *
 * The freeze window and the manual-correction layering were inline in the
 * scoreboard HTTP handler, reachable only through a database-backed request. As
 * pure functions of their inputs they can be asserted directly, which is what
 * this file does.
 *
 * Both behaviours have caused real bugs: a frozen board that leaked verdicts, and
 * a correction that moved a row without the row saying so.
 */

var (
	t0 = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	t1 = t0.Add(2 * time.Hour)
	// Mid-round, after the freeze.
	freeze = t0.Add(90 * time.Minute)
)

// boardInput is a two-problem ACM round with two participants and no rows, so
// each test adds only what it is about.
func boardInput() format.BoardInput {
	return format.BoardInput{
		ContestID:  "c1",
		StartTime:  t0,
		EndTime:    t1,
		FormatName: "acm",
		Problems: []model.ContestProblem{
			{ProblemID: "p1", Index: "A", Score: 100},
			{ProblemID: "p2", Index: "B", Score: 100},
		},
		Participants: []string{"u1", "u2"},
		Usernames:    map[string]string{"u1": "alice", "u2": "bob"},
		Now:          t0.Add(10 * time.Minute),
	}
}

func TestBuildBoardScoresAndRanks(t *testing.T) {
	in := boardInput()
	in.Rows = []model.ScoreboardRow{
		// alice solves A on the first try.
		{UserID: "u1", ProblemID: "p1", Status: "ac", Score: 100, CreatedAt: t0.Add(10 * time.Minute)},
		// bob solves A and B, but A took two wrong attempts.
		{UserID: "u2", ProblemID: "p1", Status: "wa", Score: 0, CreatedAt: t0.Add(5 * time.Minute)},
		{UserID: "u2", ProblemID: "p1", Status: "ac", Score: 100, CreatedAt: t0.Add(20 * time.Minute)},
		{UserID: "u2", ProblemID: "p2", Status: "ac", Score: 100, CreatedAt: t0.Add(30 * time.Minute)},
	}

	board, err := format.BuildBoard(in)
	if err != nil {
		t.Fatalf("BuildBoard: %v", err)
	}
	if board.Frozen {
		t.Error("the round is not past its freeze time, so Frozen must be false")
	}
	if len(board.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(board.Entries))
	}

	// bob solved both, so bob ranks first.
	if board.Entries[0].UserID != "u2" || board.Entries[0].Rank != 1 {
		t.Errorf("first entry = %s (rank %d), want u2 at rank 1",
			board.Entries[0].UserID, board.Entries[0].Rank)
	}
	if board.Entries[0].TotalSolved != 2 {
		t.Errorf("u2 TotalSolved = %d, want 2", board.Entries[0].TotalSolved)
	}
	if board.Entries[1].UserID != "u1" {
		t.Errorf("second entry = %s, want u1", board.Entries[1].UserID)
	}
	// Usernames come from the map, not from the row.
	if board.Entries[0].Username != "bob" {
		t.Errorf("u2 Username = %q, want %q", board.Entries[0].Username, "bob")
	}
}

// A judge sees the rows past the freeze; an ordinary viewer sees a count.
func TestBuildBoardFreezeWithholdsRowsButCountsActivity(t *testing.T) {
	rows := []model.ScoreboardRow{
		// Before the freeze: visible to everyone.
		{UserID: "u1", ProblemID: "p1", Status: "ac", Score: 100, CreatedAt: t0.Add(10 * time.Minute)},
		// After the freeze: a verdict only a judge may see.
		{UserID: "u2", ProblemID: "p1", Status: "ac", Score: 100, CreatedAt: freeze.Add(5 * time.Minute)},
	}

	now := freeze.Add(10 * time.Minute)

	t.Run("ordinary viewer", func(t *testing.T) {
		in := boardInput()
		in.Rows = rows
		in.FreezeTime = &freeze
		// The handler computes frozen from the wall clock; here the window is
		// established by the times themselves.
		in.StartTime, in.EndTime = t0, now.Add(time.Hour)
		in.Now = now

		board, err := format.BuildBoard(in)
		if err != nil {
			t.Fatalf("BuildBoard: %v", err)
		}
		if !board.Frozen {
			t.Fatal("the freeze window is active, so Frozen must be true")
		}

		var bob format.Entry
		for _, e := range board.Entries {
			if e.UserID == "u2" {
				bob = e
			}
		}
		// The post-freeze AC must not be scored as solved...
		if bob.Problems["A"].Solved {
			t.Error("a submission after the freeze was scored as solved for an ordinary viewer")
		}
		// ...but its existence is visible as a count.
		if bob.Problems["A"].Pending != 1 {
			t.Errorf("pending = %d, want 1 — activity after the freeze must be counted, not hidden",
				bob.Problems["A"].Pending)
		}
	})

	t.Run("judge", func(t *testing.T) {
		in := boardInput()
		in.Rows = rows
		in.FreezeTime = &freeze
		in.StartTime, in.EndTime = t0, now.Add(time.Hour)
		in.Now = now
		in.ViewPastFreeze = true

		board, err := format.BuildBoard(in)
		if err != nil {
			t.Fatalf("BuildBoard: %v", err)
		}
		var bob format.Entry
		for _, e := range board.Entries {
			if e.UserID == "u2" {
				bob = e
			}
		}
		if !bob.Problems["A"].Solved {
			t.Error("a judge must see the post-freeze submission as solved")
		}
		if bob.Problems["A"].Pending != 0 {
			t.Errorf("pending = %d, want 0 — a judge is given the rows, not a count",
				bob.Problems["A"].Pending)
		}
	})
}

// A correction moves the standing and marks the row.
func TestBuildBoardAppliesCorrectionsBeforeRanking(t *testing.T) {
	in := boardInput()
	// alice and bob both solve exactly one problem, so alice wins on time
	// without any correction.
	in.Rows = []model.ScoreboardRow{
		{UserID: "u1", ProblemID: "p1", Status: "ac", Score: 100, CreatedAt: t0.Add(5 * time.Minute)},
		{UserID: "u2", ProblemID: "p1", Status: "ac", Score: 100, CreatedAt: t0.Add(50 * time.Minute)},
	}
	in.Corrections = map[string]model.AdjustmentTotals{
		// A judge credits bob with a second problem.
		"u2": {PointDelta: 100, SolvedDelta: 1},
	}

	board, err := format.BuildBoard(in)
	if err != nil {
		t.Fatalf("BuildBoard: %v", err)
	}

	if board.Entries[0].UserID != "u2" {
		t.Fatalf("first entry = %s, want u2 — the correction must be applied before ranking",
			board.Entries[0].UserID)
	}
	if board.Entries[0].TotalScore != 101 {
		t.Errorf("u2 TotalScore = %d, want 101 — ACM awards 1 per solved problem, so 1 + a +100 correction",
			board.Entries[0].TotalScore)
	}
	// The row must carry the correction so a reader can tell the number is not
	// purely the judge's output.
	if board.Entries[0].Adjustment == nil {
		t.Error("u2 has no Adjustment on its row, so an adjusted standing is indistinguishable from a computed one")
	}
	// The participant with no correction must not be marked.
	if board.Entries[1].Adjustment != nil {
		t.Errorf("u1 Adjustment = %+v, want nil", board.Entries[1].Adjustment)
	}
}

// A correction can never push a count below zero, or a bad row would rank better
// than solving nothing.
func TestBuildBoardCorrectionFloorsAtZero(t *testing.T) {
	in := boardInput()
	in.Rows = nil
	in.Corrections = map[string]model.AdjustmentTotals{
		"u1": {PointDelta: -50, SolvedDelta: -3, PenaltyDelta: -100},
	}

	board, err := format.BuildBoard(in)
	if err != nil {
		t.Fatalf("BuildBoard: %v", err)
	}
	for _, e := range board.Entries {
		if e.UserID != "u1" {
			continue
		}
		if e.TotalSolved < 0 {
			t.Errorf("TotalSolved = %d after a negative correction, want floored at 0", e.TotalSolved)
		}
		if e.TotalPenalty < 0 {
			t.Errorf("TotalPenalty = %d after a negative correction, want floored at 0", e.TotalPenalty)
		}
	}
}

// An empty round is a valid empty board, not an error.
func TestBuildBoardEmptyRound(t *testing.T) {
	board, err := format.BuildBoard(format.BoardInput{
		FormatName: "acm", StartTime: t0, EndTime: t1,
	})
	if err != nil {
		t.Fatalf("BuildBoard on an empty round: %v", err)
	}
	if len(board.Entries) != 0 {
		t.Errorf("got %d entries, want 0", len(board.Entries))
	}
}

// A format name that is not registered is an error, not a panic or a silent
// empty board: the name comes from stored contest data.
func TestBuildBoardUnknownFormat(t *testing.T) {
	if _, err := format.BuildBoard(format.BoardInput{FormatName: "no-such-format"}); err == nil {
		t.Error("an unregistered format name must be an error")
	}
}

// Display labels are attached per row: a generated onsite account shows its team,
// an ordinary user shows their username.
func TestBuildBoardAttachesDisplayLabels(t *testing.T) {
	in := boardInput()
	in.Participants = []string{"u1"}
	in.Usernames = map[string]string{"u1": "team_9f3a"}
	in.DisplayNames = map[string]model.ContestDisplayName{
		"u1": {TeamName: "Team Rocket", Institution: "MIT"},
	}

	board, err := format.BuildBoard(in)
	if err != nil {
		t.Fatalf("BuildBoard: %v", err)
	}
	e := board.Entries[0]
	if e.TeamName != "Team Rocket" || e.Institution != "MIT" {
		t.Errorf("labels = %q / %q, want %q / %q", e.TeamName, e.Institution, "Team Rocket", "MIT")
	}
	// The handle is still carried; the label is additive, not a replacement.
	if e.Username != "team_9f3a" {
		t.Errorf("Username = %q, want the handle", e.Username)
	}
}

// TestEntryWireShape pins the JSON contract.
//
// Entry used to be declared inside the scoreboard handler with explicit JSON
// tags. Moving it here without them would have renamed every field to its Go name
// — Rank, UserID, TotalSolved — and the scoreboard reads rank, user_id and
// total_solved. The frontend would have rendered an empty table, and no Go test
// that only inspects the struct would have noticed.
func TestEntryWireShape(t *testing.T) {
	in := boardInput()
	in.Participants = []string{"u1"}
	in.Rows = []model.ScoreboardRow{
		{UserID: "u1", ProblemID: "p1", Status: "ac", Score: 1, CreatedAt: t0.Add(time.Minute)},
	}

	board, err := format.BuildBoard(in)
	if err != nil {
		t.Fatalf("BuildBoard: %v", err)
	}
	raw, err := json.Marshal(board.Entries[0])
	if err != nil {
		t.Fatalf("marshal entry: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal entry: %v", err)
	}
	for _, key := range []string{
		"rank", "user_id", "username", "total_solved", "total_penalty",
		"total_score", "problems",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("entry JSON has no %q key, so the scoreboard cannot read it: %s", key, raw)
		}
	}
	// The Go field names must NOT appear.
	for _, key := range []string{"Rank", "UserID", "TotalSolved"} {
		if _, ok := got[key]; ok {
			t.Errorf("entry JSON has %q — the wire shape leaked a Go field name: %s", key, raw)
		}
	}
	// The per-problem cell keeps its own contract.
	var cells map[string]map[string]json.RawMessage
	if err := json.Unmarshal(got["problems"], &cells); err != nil {
		t.Fatalf("unmarshal problems: %v", err)
	}
	cell, ok := cells["A"]
	if !ok {
		t.Fatalf("no cell for problem A: %s", got["problems"])
	}
	for _, key := range []string{"solved", "attempts", "time", "score", "pending"} {
		if _, ok := cell[key]; !ok {
			t.Errorf("cell JSON has no %q key: %s", key, got["problems"])
		}
	}
}

// A round that has already ended is not frozen, even though its freeze time is
// in the past.
//
// The rule the handler applied was "is the clock past the freeze AND before the
// end". Reading it as "does the freeze fall inside the window" gives a different
// answer for an ended round, and would withhold rows from a board whose round is
// over — which is the one moment a standings page must show everything.
func TestBuildBoardEndedRoundIsNotFrozen(t *testing.T) {
	freezeTime := t0.Add(90 * time.Minute)
	ended := t0.Add(2 * time.Hour).Add(time.Second)

	in := boardInput()
	in.StartTime, in.EndTime = t0, t0.Add(2*time.Hour)
	in.FreezeTime = &freezeTime
	in.Now = ended
	in.Rows = []model.ScoreboardRow{
		{UserID: "u1", ProblemID: "p1", Status: "ac", Score: 1, CreatedAt: freezeTime.Add(time.Minute)},
	}

	board, err := format.BuildBoard(in)
	if err != nil {
		t.Fatalf("BuildBoard: %v", err)
	}
	if board.Frozen {
		t.Error("an ended round is not frozen, so Frozen must be false")
	}
	if !board.Entries[0].Problems["A"].Solved {
		t.Error("an ended round's standings must score the post-freeze submission")
	}
}

// A caller that does not set Now gets no freeze rather than an always-frozen
// board, which would hide every row from everyone.
func TestBuildBoardWithoutClockIsNotFrozen(t *testing.T) {
	freezeTime := t0.Add(time.Minute)
	in := boardInput()
	in.FreezeTime = &freezeTime
	// boardInput sets Now to a real instant; zero it so this is the no-clock case.
	in.Now = time.Time{}

	board, err := format.BuildBoard(in)
	if err != nil {
		t.Fatalf("BuildBoard: %v", err)
	}
	if board.Frozen {
		t.Error("a caller that set no Now must not get a frozen board")
	}
}
