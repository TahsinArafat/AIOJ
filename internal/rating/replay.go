package rating

import "github.com/tahsinarafat/aioj/internal/model"

/**
 * Suffix replay.
 *
 * The rating algorithm is a fold: each round's base is the previous round's
 * result. That makes a single round impossible to withdraw on its own — every
 * later round was computed on top of it — so a dispute is answered by recomputing
 * the suffix from the disputed round onward, in calendar order.
 *
 * Everything here is a pure function of its inputs. The caller supplies the rounds
 * in order and the ratings the users entered the first of them with; nothing
 * reaches a store, which is what lets a replay be asserted against a literal
 * sequence instead of a database.
 */

// RosterEntry is one participant's result in one round: the rank they finished
// on, and the name to address them by.
type RosterEntry struct {
	UserID   string
	Username string
	Rank     int
}

// Round is one rated round in the replay, in the order it was contested.
type Round struct {
	ContestID string
	// Roster is the round's rated field in rank order.
	Roster []RosterEntry
	// Voided means the round no longer counts: it contributes no rows and moves
	// nobody's rating, and the rounds after it are replayed as though it had never
	// been rated.
	Voided bool
	// Min and Max are the round's rating window. The window is tested against the
	// rating a user carries INTO the round, which is why it is applied here and not
	// when the caller builds the roster: during a replay that rating is the
	// previous round's result, not anything stored on the round.
	Min int
	Max int
}

// RoundOutcome is one round's recomputed result.
type RoundOutcome struct {
	ContestID string
	Changes   []model.RatingChange
}

// Replay recomputes ratings across a suffix of rounds.
//
// in.Base is each user's rating before the first round of the suffix; a user
// absent from it is unrated and enters at DefaultRating, the same rule the first
// payout uses. The returned map is each user's rating after the last round, which
// is what the caller writes back to the profiles it touched.
func Replay(rounds []Round, base map[string]int) ([]RoundOutcome, map[string]int) {
	carried := make(map[string]int, len(base))
	for userID, r := range base {
		carried[userID] = r
	}

	outcomes := make([]RoundOutcome, 0, len(rounds))
	for _, round := range rounds {
		if round.Voided {
			// No rows at all, not rows of zero change: the round is not part of the
			// ledger while it is voided, and an empty change set is how the caller
			// knows to leave it out.
			outcomes = append(outcomes, RoundOutcome{ContestID: round.ContestID})
			continue
		}

		standings := make([]ContestStanding, 0, len(round.Roster))
		for _, entry := range round.Roster {
			standings = append(standings, ContestStanding{
				UserID:    entry.UserID,
				Rank:      entry.Rank,
				OldRating: carried[entry.UserID],
				Username:  entry.Username,
			})
		}

		// An entrant outside the window does not contest the round, and leaving
		// them in would count them in the field size and leave holes in the ranks.
		standings = RatedField(standings, round.Min, round.Max)
		participants := len(standings)
		changes := make([]model.RatingChange, 0, participants)
		for _, standing := range standings {
			oldRating := effectiveRating(standing.OldRating)
			newRating := CalculateRating(oldRating, standing.Rank, participants)
			carried[standing.UserID] = newRating

			changes = append(changes, model.RatingChange{
				UserID:       standing.UserID,
				Username:     standing.Username,
				OldRating:    oldRating,
				NewRating:    newRating,
				RatingChange: CalculateRatingChange(oldRating, newRating),
				Rank:         standing.Rank,
				Color:        model.GetColor(newRating),
			})
		}

		outcomes = append(outcomes, RoundOutcome{ContestID: round.ContestID, Changes: changes})
	}

	return outcomes, carried
}
