package format

import (
	"strings"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
)

/**
 * Standings assembly.
 *
 * These pieces were inline in the scoreboard HTTP handler: partition the judged
 * rows into per-participant-per-problem buckets, apply the freeze cutoff, score
 * every cell, layer the jury's manual corrections over the computed totals,
 * rank, and project the wire shape. That is the round's scoring policy, and it
 * sat in a 320-line handler next to cache-key construction, HTTP header setting
 * and pagination arithmetic.
 *
 * None of it needs an *http.Request. Moving it here is what makes it testable
 * without a database: a scoring rule can be asserted against a slice of rows,
 * which is how the freeze and adjustment behaviour below is pinned.
 *
 * The row and adjustment types come from the model package rather than being
 * redeclared, because they cross the store boundary — the contest store returns
 * them and this module consumes them. format is otherwise dependency-free apart
 * from this, which the format implementations' init() registration does not
 * depend on.
 */

// BoardInput is everything scoring needs about one round. The handler fetches
// these; nothing here reaches a store.
type BoardInput struct {
	ContestID  string
	StartTime  time.Time
	EndTime    time.Time
	FormatName string
	// FormatConfig is the round's per-format configuration, passed through to
	// each ScoreProblem call.
	FormatConfig []byte

	// Problems is the round's roster, in board order.
	Problems []model.ContestProblem
	// Rows is every judged submission the standings read.
	Rows []model.ScoreboardRow
	// Participants is the user ids the board ranks, in the order the store
	// returned them.
	Participants []string

	// Usernames maps user id to the handle to display. A missing key yields an
	// empty username, which is what the single-row lookup produced.
	Usernames map[string]string
	// DisplayNames maps user id to the team label a generated onsite account is
	// shown under. Separate from Usernames because they come from different
	// queries: the handle is on the users row, the team on the onsite batch row.
	// A missing key means the row has no team label and the username stands.
	DisplayNames map[string]model.ContestDisplayName

	// FreezeTime is when the standings stop advancing for ordinary viewers.
	// nil means the round never freezes.
	FreezeTime *time.Time
	// Now is the moment the freeze is evaluated against. The handler passes
	// time.Now(); a test passes a fixed instant. Required to reproduce the
	// original rule exactly — "is it currently past the freeze" is not the same
	// question as "does the freeze fall inside the window", and the two disagree
	// for a round that has already ended or whose freeze is set past its end.
	Now time.Time
	// ViewPastFreeze reports that this caller is a judge, and so sees the rows
	// the freeze would otherwise withhold.
	ViewPastFreeze bool

	// Corrections is the jury's manual deltas, keyed by user id.
	Corrections map[string]model.AdjustmentTotals
}

// Cell is one participant's result on one problem. It is model.ProblemResult
// rather than a local type because that is already the wire shape the standings
// table renders, and a second declaration would be a second thing to keep in
// step with the frontend.

// Entry is one ranked row of the standings.
//
// The JSON tags are the API contract, not decoration. This type used to be
// declared inside the handler with these tags; moving it without them would have
// renamed every field to its Go name (Rank, UserID, TotalSolved) and broken the
// scoreboard, which reads rank, user_id and total_solved.
type Entry struct {
	Rank         int                            `json:"rank"`
	UserID       string                         `json:"user_id"`
	Username     string                         `json:"username"`
	TeamName     string                         `json:"team_name,omitempty"`
	Institution  string                         `json:"institution,omitempty"`
	TotalSolved  int                            `json:"total_solved"`
	TotalPenalty int                            `json:"total_penalty"`
	TotalScore   int                            `json:"total_score"`
	Problems     map[string]model.ProblemResult `json:"problems"`
	// Adjustment is set when a correction was applied to this participant, so
	// the row can be marked and a reader knows the numbers are not purely the
	// judge's output.
	Adjustment *model.AdjustmentTotals `json:"adjustment,omitempty"`
}

// Board is a fully scored and ranked standings table.
type Board struct {
	Entries []Entry
	// Frozen reports whether the freeze window was active for this request.
	Frozen bool
}

// BuildBoard scores every participant against every problem, applies the freeze,
// layers the corrections, and ranks the result.
//
// The returned error is only ever a scoring failure — a format implementation
// refusing its inputs, or a round whose format name is not registered. A round
// with no participants, no problems or no rows is a valid empty board, not an
// error.
func BuildBoard(in BoardInput) (*Board, error) {
	f, err := Create(in.FormatName, in.FormatConfig)
	if err != nil {
		return nil, err
	}

	// The same rule the handler applied: the board is frozen once the clock is
	// past the freeze time and the round is still running. isZero covers a caller
	// that did not set Now, which would otherwise make every round look frozen.
	frozen := in.FreezeTime != nil && !in.Now.IsZero() &&
		in.Now.After(*in.FreezeTime) && in.Now.Before(in.EndTime)
	// The cutoff only applies to a viewer who may not see past it.
	hideFrozen := frozen && !in.ViewPastFreeze

	// A submission carries the problem's id; a frozen count is read back with
	// the letter the board already shows. Translate once, here, or the two
	// identifiers never meet and every pending count reports zero.
	indexByID := make(map[string]string, len(in.Problems))
	for _, p := range in.Problems {
		indexByID[p.ProblemID] = p.Index
	}

	type userProblemKey struct {
		UserID    string
		ProblemID string
	}
	type userLetterKey struct {
		UserID string
		Letter string
	}

	scored := make(map[userProblemKey][]Submission)
	pending := make(map[userLetterKey]int)

	var cutoff time.Time
	if hideFrozen && in.FreezeTime != nil {
		cutoff = *in.FreezeTime
	}
	for _, row := range in.Rows {
		if hideFrozen && row.CreatedAt.After(cutoff) {
			// Counted, not scored: the cell shows how much happened after the
			// freeze without saying what the verdict was.
			pending[userLetterKey{UserID: row.UserID, Letter: indexByID[row.ProblemID]}]++
			continue
		}
		key := userProblemKey{UserID: row.UserID, ProblemID: row.ProblemID}
		scored[key] = append(scored[key], Submission{
			UserID:    row.UserID,
			ProblemID: row.ProblemID,
			// Upper-cased here because the formats compare against "AC" exactly.
			// The database stores the verdict enum lowercase ('ac'), so passing it
			// through unnormalized makes every submission look unsolved. This
			// normalization used to live in the HTTP handler; it belongs with the
			// scoring, and board_test.go fails without it.
			Status:    strings.ToUpper(row.Status),
			Score:     float64(row.Score),
			CreatedAt: row.CreatedAt,
		})
	}

	formatProblems := make([]Problem, len(in.Problems))
	for i, p := range in.Problems {
		formatProblems[i] = Problem{ID: p.ProblemID, Index: p.Index}
	}

	scores := make([]ParticipantScore, 0, len(in.Participants))
	adjusted := make(map[string]model.AdjustmentTotals, len(in.Corrections))

	for _, uid := range in.Participants {
		ps := ParticipantScore{
			UserID:   uid,
			Username: in.Usernames[uid],
			Problems: make([]ProblemResult, 0, len(in.Problems)),
		}

		for _, problem := range formatProblems {
			result, err := f.ScoreProblem(ScoringContext{
				ContestID:           in.ContestID,
				ContestDuration:     in.EndTime.Sub(in.StartTime),
				SubmissionStartTime: in.StartTime,
				Problem:             problem,
				Submissions:         scored[userProblemKey{UserID: uid, ProblemID: problem.ID}],
				FormatConfig:        in.FormatConfig,
			})
			if err != nil {
				return nil, err
			}
			ps.Problems = append(ps.Problems, result)
			if result.Solved {
				ps.TotalSolved++
			}
			ps.TotalScore += result.Score
			ps.TotalPenalty += result.Penalty
		}

		// Layer the correction over the computed totals. It is applied before
		// ranking, so a judge's decision actually moves the order.
		if c, ok := in.Corrections[uid]; ok && !c.IsZero() {
			ps.TotalScore += float64(c.PointDelta)
			ps.TotalSolved += c.SolvedDelta
			ps.TotalPenalty += c.PenaltyDelta
			// TotalSolved counts problems, so a correction can never push it
			// below zero — a negative count would rank better than solving
			// nothing. The same floor applies to the penalty.
			if ps.TotalSolved < 0 {
				ps.TotalSolved = 0
			}
			if ps.TotalPenalty < 0 {
				ps.TotalPenalty = 0
			}
			adjusted[uid] = c
		}

		scores = append(scores, ps)
	}

	ranks := f.RankParticipants(scores)

	entries := make([]Entry, len(ranks))
	for i, rank := range ranks {
		problems := make(map[string]model.ProblemResult, len(rank.Score.Problems))
		for _, pr := range rank.Score.Problems {
			problems[pr.ProblemIndex] = model.ProblemResult{
				Solved:   pr.Solved,
				Attempts: pr.Attempts,
				Time:     pr.Penalty,
				Score:    int(pr.Score),
				Pending:  pending[userLetterKey{UserID: rank.Score.UserID, Letter: pr.ProblemIndex}],
			}
		}
		var adj *model.AdjustmentTotals
		if c, ok := adjusted[rank.Score.UserID]; ok {
			copied := c
			adj = &copied
		}
		dn := in.DisplayNames[rank.Score.UserID]
		entries[i] = Entry{
			Rank:         rank.Position,
			UserID:       rank.Score.UserID,
			Username:     rank.Score.Username,
			TeamName:     dn.TeamName,
			Institution:  dn.Institution,
			TotalSolved:  rank.Score.TotalSolved,
			TotalPenalty: rank.Score.TotalPenalty,
			TotalScore:   int(rank.Score.TotalScore),
			Problems:     problems,
			Adjustment:   adj,
		}
	}

	return &Board{Entries: entries, Frozen: frozen}, nil
}
