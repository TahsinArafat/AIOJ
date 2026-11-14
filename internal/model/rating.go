package model

import "time"

// Rating color tiers
const (
	ColorNovice      = "novice"      // < 1200
	ColorApprentice  = "apprentice"  // 1200-1399
	ColorAdept       = "adept"       // 1400-1599
	ColorElite       = "elite"       // 1600-1899
	ColorChampion    = "champion"    // 1900-2099
	ColorMaster      = "master"      // 2100-2299
	ColorGrandmaster = "grandmaster" // 2300-2399
	ColorTitan       = "titan"       // 2400-2599
	ColorImmortal    = "immortal"    // 2600-2899
	ColorApex        = "apex"        // 2900+
)

// RatingHistory is one participant's result in one round, as the ledger records
// it. It is not a log of edits: a recalculation supersedes these rows and writes
// new ones, so a row is in force exactly when SupersededBy is empty.
type RatingHistory struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	ContestID string `json:"contest_id"`
	// RunID is the payout that produced this row. One run covers every
	// participant of a round, so reversing a round is one run to undo rather than
	// N rows to find.
	RunID string `json:"run_id"`
	// SupersededBy is the run that replaced this row. Empty means in force.
	SupersededBy string `json:"superseded_by,omitempty"`
	// Seq orders rows by insertion. created_at cannot: two runs in the same
	// millisecond tie, and replaying a suffix depends on a total order.
	Seq          int64     `json:"seq"`
	OldRating    int       `json:"old_rating"`
	NewRating    int       `json:"new_rating"`
	Rank         int       `json:"rank"`
	RatingChange int       `json:"rating_change"`
	CreatedAt    time.Time `json:"created_at"`
}

// RatingRun is one payout: the round it was for, who ran it, and why.
type RatingRun struct {
	ID         string    `json:"id"`
	ContestID  string    `json:"contest_id"`
	ActorID    string    `json:"actor_id,omitempty"`
	ActorName  string    `json:"actor_name"`
	Kind       string    `json:"kind"`
	Reason     string    `json:"reason"`
	ReplacesID string    `json:"replaces_run_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Rating run kinds.
const (
	// RatingRunApply is the first payout of a round.
	RatingRunApply = "apply"
	// RatingRunRecalc replaces an earlier payout: a disputed round voided, an
	// entrant disqualified, or a suffix replayed after either.
	RatingRunRecalc = "recalc"
)

// RatingExclusion is one entrant disqualified from one round. The round still
// counts for everyone else; this entrant's result in it does not, which removes
// them from the field and re-ranks the rest.
type RatingExclusion struct {
	UserID    string    `json:"user_id"`
	ContestID string    `json:"contest_id"`
	Reason    string    `json:"reason"`
	CreatedBy string    `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type RatingChange struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	OldRating    int    `json:"old_rating"`
	NewRating    int    `json:"new_rating"`
	RatingChange int    `json:"rating_change"`
	Rank         int    `json:"rank"`
	Color        string `json:"color"`
}

type ContestRatingRequest struct {
	ContestID string `json:"contest_id"`
}

// GetColor returns the color name for a given rating
func GetColor(rating int) string {
	switch {
	case rating >= 2900:
		return ColorApex
	case rating >= 2600:
		return ColorImmortal
	case rating >= 2400:
		return ColorTitan
	case rating >= 2300:
		return ColorGrandmaster
	case rating >= 2100:
		return ColorMaster
	case rating >= 1900:
		return ColorChampion
	case rating >= 1600:
		return ColorElite
	case rating >= 1400:
		return ColorAdept
	case rating >= 1200:
		return ColorApprentice
	default:
		return ColorNovice
	}
}

// GetColorHex returns the hex color for a given rating
func GetColorHex(rating int) string {
	switch {
	case rating >= 2900:
		return "#FF0000"
	case rating >= 2600:
		return "#FF0000"
	case rating >= 2400:
		return "#FF8C00"
	case rating >= 2300:
		return "#FF8C00"
	case rating >= 2100:
		return "#FFD700"
	case rating >= 1900:
		return "#AA00AA"
	case rating >= 1600:
		return "#0000FF"
	case rating >= 1400:
		return "#03A89E"
	case rating >= 1200:
		return "#008000"
	default:
		return "#808080"
	}
}
