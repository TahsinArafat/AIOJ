package format

import (
	"encoding/json"
	"strings"
	"time"
)

// StatusSkipped is a submission an organizer voided. The formats receive
// UPPER-CASE statuses (the handler upper-cases the stored enum before scoring),
// so this is the spelling they compare against.
//
// A skipped submission is not an attempt: Codeforces drops it from the
// wrong-attempt count and the penalty that follows from it, so a round is
// scored as if it had never been made. Every format must therefore filter it
// out BEFORE counting, not merely refuse to accept on it.
const StatusSkipped = "SKIPPED"

// IsSkipped reports whether a submission was voided by an organizer.
func IsSkipped(status string) bool {
	return strings.EqualFold(status, StatusSkipped)
}

// ScoringSubmissions returns the submissions a format may score: the ones an
// organizer has not voided.
//
// Every algorithm must start from this list, and that is the whole point of it
// living here. "A skipped submission does not exist as far as the round is
// concerned" is one rule, but a per-algorithm loop restating it is a rule that
// gets forgotten — the score-threshold formats (ioi, oi) are the trap, because
// they read a submission's stored score directly and a voided row would keep
// paying out its points even though it no longer counts as an attempt.
//
// The result is always a fresh slice: the caller's slice is shared with the
// scoreboard handler's per-problem map and must not be mutated in place.
func ScoringSubmissions(subs []Submission) []Submission {
	out := make([]Submission, 0, len(subs))
	for _, s := range subs {
		if !IsSkipped(s.Status) {
			out = append(out, s)
		}
	}
	return out
}

type Submission struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ProblemID string    `json:"problem_id"`
	Status    string    `json:"status"`
	Score     float64   `json:"score"`
	CreatedAt time.Time `json:"created_at"`
}

type Problem struct {
	ID     string          `json:"id"`
	Index  string          `json:"index"`
	Config json.RawMessage `json:"config,omitempty"`
}

type ScoringContext struct {
	ContestID           string          `json:"contest_id"`
	ContestDuration     time.Duration   `json:"contest_duration"`
	SubmissionStartTime time.Time       `json:"submission_start_time"`
	Problem             Problem         `json:"problem"`
	Submissions         []Submission    `json:"submissions"`
	FormatConfig        json.RawMessage `json:"format_config"`
}

type ProblemResult struct {
	ProblemID     string             `json:"problem_id"`
	ProblemIndex  string             `json:"problem_index"`
	Solved        bool               `json:"solved"`
	Score         float64            `json:"score"`
	Attempts      int                `json:"attempts"`
	Penalty       int                `json:"penalty"`
	FirstACTime   *time.Time         `json:"first_ac_time,omitempty"`
	SubtaskScores map[string]float64 `json:"subtask_scores,omitempty"`
}

type ParticipantScore struct {
	UserID       string          `json:"user_id"`
	Username     string          `json:"username"`
	Problems     []ProblemResult `json:"problems"`
	TotalSolved  int             `json:"total_solved"`
	TotalScore   float64         `json:"total_score"`
	TotalPenalty int             `json:"total_penalty"`
}

type Rank struct {
	Position int              `json:"position"`
	Score    ParticipantScore `json:"score"`
}

type ContestFormat interface {
	Name() string
	ScoreProblem(ctx ScoringContext) (ProblemResult, error)
	RankParticipants(participants []ParticipantScore) []Rank
	ValidateConfig(config json.RawMessage) error
	DefaultConfig() json.RawMessage
}
