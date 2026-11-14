package rating

import "testing"

func roster(ranks ...string) []RosterEntry {
	out := make([]RosterEntry, len(ranks))
	for i, userID := range ranks {
		out[i] = RosterEntry{UserID: userID, Username: userID, Rank: i + 1}
	}
	return out
}

// A voided round moves nobody and leaves the following round to be computed from
// the rating before it, which is the whole reason a replay walks the suffix.
func TestReplayVoidedRoundContributesNothing(t *testing.T) {
	outcomes, final := Replay([]Round{
		{ContestID: "c1", Roster: roster("a", "b", "c", "d"), Voided: true},
		{ContestID: "c2", Roster: roster("a", "b", "c", "d"), Max: 9999},
	}, nil)

	if len(outcomes) != 2 {
		t.Fatalf("expected 2 outcomes, got %d", len(outcomes))
	}
	if len(outcomes[0].Changes) != 0 {
		t.Errorf("voided round produced %d changes, want 0", len(outcomes[0].Changes))
	}
	if len(outcomes[1].Changes) != 4 {
		t.Fatalf("second round produced %d changes, want 4", len(outcomes[1].Changes))
	}
	// The first place in round 2 must start from the newcomer baseline, not from
	// anything the voided round would have given them.
	if got := outcomes[1].Changes[0].OldRating; got != DefaultRating {
		t.Errorf("round 2 base = %d, want %d", got, DefaultRating)
	}
	if final["a"] != outcomes[1].Changes[0].NewRating {
		t.Errorf("final rating for a = %d, want %d", final["a"], outcomes[1].Changes[0].NewRating)
	}
}

// The fold is the reason a single round cannot be recomputed alone: round 2's base
// is round 1's result, so replaying both must differ from replaying round 2 alone.
func TestReplayFoldsAcrossRounds(t *testing.T) {
	rounds := []Round{
		{ContestID: "c1", Roster: roster("a", "b", "c", "d"), Max: 9999},
		{ContestID: "c2", Roster: roster("a", "b", "c", "d"), Max: 9999},
	}

	_, both := Replay(rounds, nil)
	_, secondOnly := Replay(rounds[1:], nil)
	_, secondFromWinner := Replay(rounds[1:], map[string]int{"a": both["a"]})

	if both["a"] == secondOnly["a"] {
		t.Fatalf("replaying both rounds gave the same rating (%d) as replaying the second alone", both["a"])
	}
	if secondFromWinner["a"] == secondOnly["a"] {
		t.Errorf("a different starting rating produced the same result (%d)", secondOnly["a"])
	}
}

// The window is tested against the rating a user carries into the round, which
// only exists during the replay.
func TestReplayAppliesWindowToCarriedRating(t *testing.T) {
	rounds := []Round{
		{ContestID: "c1", Roster: roster("a", "b", "c", "d"), Min: 1600, Max: 9999},
	}

	outcomes, _ := Replay(rounds, map[string]int{"a": 1700, "b": 1500, "c": 1650, "d": 1200})

	if len(outcomes[0].Changes) != 2 {
		t.Fatalf("expected 2 entrants inside 1600+, got %d", len(outcomes[0].Changes))
	}
	for _, change := range outcomes[0].Changes {
		if change.UserID != "a" && change.UserID != "c" {
			t.Errorf("%s was rated but is outside the window", change.UserID)
		}
	}
	// The survivors are re-ranked from 1: the ineligible entrants must not leave
	// holes that make them look beaten by people who were not in the field.
	if outcomes[0].Changes[0].UserID != "a" || outcomes[0].Changes[0].Rank != 1 {
		t.Errorf("first rated entrant = %s at rank %d, want a at rank 1",
			outcomes[0].Changes[0].UserID, outcomes[0].Changes[0].Rank)
	}
}

// Ties keep their rank in the replay too, so two entrants on the same board rank
// are scored the same.
func TestReplayKeepsTies(t *testing.T) {
	rounds := []Round{{
		ContestID: "c1",
		Roster: []RosterEntry{
			{UserID: "a", Rank: 1},
			{UserID: "b", Rank: 1},
			{UserID: "c", Rank: 3},
		},
		Max: 9999,
	}}

	outcomes, _ := Replay(rounds, nil)
	changes := outcomes[0].Changes
	if changes[0].Rank != 1 || changes[1].Rank != 1 {
		t.Errorf("tied entrants got ranks %d and %d, want 1 and 1", changes[0].Rank, changes[1].Rank)
	}
	if changes[2].Rank != 3 {
		t.Errorf("third entrant rank = %d, want 3", changes[2].Rank)
	}
}
