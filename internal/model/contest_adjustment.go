package model

import "time"

// Adjustment scopes. A correction is applied at exactly one level, and the scope
// is what the scoreboard uses to decide where the delta lands.
const (
	// AdjustmentContest corrects the whole round — a ruling that everyone gets
	// the points for a broken problem.
	AdjustmentContest = "contest"
	// AdjustmentProblem corrects one problem's column for one participant.
	AdjustmentProblem = "problem"
	// AdjustmentSubmission corrects the result attributed to a single submission.
	AdjustmentSubmission = "submission"
)

// ContestAdjustment is a manual correction to a participant's standing.
//
// It is a signed delta, never a replacement: the submissions keep the verdicts
// they actually received, the format's scoring math is unchanged, and this is
// layered on top. That is what makes a corrected round auditable — you can
// always recompute the computer's answer and see the human one beside it.
type ContestAdjustment struct {
	ID        string `json:"id"`
	ContestID string `json:"contest_id"`
	UserID    string `json:"user_id"`
	// Username is filled in by the list query so the panel can name the person
	// without a second round trip.
	Username string `json:"username,omitempty"`
	// TeamName is the onsite team an account was minted for, when it has one.
	TeamName string `json:"team_name,omitempty"`
	Scope    string `json:"scope"`
	// ProblemID is set for a 'problem' scope, and for 'submission' it names the
	// problem the scored submission belongs to.
	ProblemID *string `json:"problem_id,omitempty"`
	// ProblemIndex is the contest letter (A, B, …), which is how a round is read.
	ProblemIndex *string `json:"problem_index,omitempty"`
	SubmissionID *string `json:"submission_id,omitempty"`
	PointDelta   int     `json:"point_delta"`
	SolvedDelta  int     `json:"solved_delta"`
	PenaltyDelta int     `json:"penalty_delta"`
	// Reason is required: a correction nobody can explain later is worse than
	// the wrong result it replaced.
	Reason    string    `json:"reason"`
	CreatedBy *string   `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// AdjustmentTotals is the sum of one participant's adjustments in a contest.
// The scoreboard applies this to the computed totals.
type AdjustmentTotals struct {
	PointDelta   int
	SolvedDelta  int
	PenaltyDelta int
}

// IsZero reports whether applying these totals would change nothing.
func (a AdjustmentTotals) IsZero() bool {
	return a.PointDelta == 0 && a.SolvedDelta == 0 && a.PenaltyDelta == 0
}

// ContestAdjustmentRequest is the create payload.
type ContestAdjustmentRequest struct {
	UserID       string  `json:"user_id"`
	Scope        string  `json:"scope"`
	ProblemID    *string `json:"problem_id,omitempty"`
	SubmissionID *string `json:"submission_id,omitempty"`
	PointDelta   int     `json:"point_delta"`
	SolvedDelta  int     `json:"solved_delta"`
	PenaltyDelta int     `json:"penalty_delta"`
	Reason       string  `json:"reason"`
}

// Validate checks a create request is usable before it reaches the database, so
// the caller gets a message instead of a constraint violation.
func (r ContestAdjustmentRequest) Validate() string {
	if r.UserID == "" {
		return "user_id is required"
	}
	switch r.Scope {
	case AdjustmentContest:
		if r.ProblemID != nil || r.SubmissionID != nil {
			return "a contest-wide adjustment must not name a problem or submission"
		}
	case AdjustmentProblem:
		if r.ProblemID == nil || *r.ProblemID == "" {
			return "a problem adjustment must name problem_id"
		}
		if r.SubmissionID != nil {
			return "a problem adjustment must not name a submission"
		}
	case AdjustmentSubmission:
		if r.SubmissionID == nil || *r.SubmissionID == "" {
			return "a submission adjustment must name submission_id"
		}
	default:
		return "scope must be one of: contest, problem, submission"
	}
	if r.PointDelta == 0 && r.SolvedDelta == 0 && r.PenaltyDelta == 0 {
		return "at least one of point_delta, solved_delta or penalty_delta must be non-zero"
	}
	if len(r.Reason) < 3 {
		return "reason is required"
	}
	return ""
}
