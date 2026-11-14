package model

import "time"

type SubmissionStatus string

const (
	StatusPending   SubmissionStatus = "pending"
	StatusJudging   SubmissionStatus = "judging"
	StatusRejudging SubmissionStatus = "rejudging"
	StatusAC        SubmissionStatus = "ac"
	StatusWA        SubmissionStatus = "wa"
	StatusTLE       SubmissionStatus = "tle"
	StatusMLE       SubmissionStatus = "mle"
	StatusRE        SubmissionStatus = "re"
	StatusCE        SubmissionStatus = "ce"
	StatusSE        SubmissionStatus = "se"
	// StatusSkipped is a human verdict, not a judge one: an organizer voids the
	// submission (a resubmit-rule breach, a copied solution). It scores nothing
	// and is not counted as an attempt, so the round is scored exactly as if the
	// submission had not been made. Nothing in the judge ever writes it.
	StatusSkipped SubmissionStatus = "skipped"
)

// Submission types
const (
	SubmissionTypeCode   = "code"
	SubmissionTypeOutput = "output"
)

// StatusAccepted is an alias for StatusAC for readability in judge code
const StatusAccepted = StatusAC
const StatusWrongAnswer = StatusWA
const StatusTimeLimitExceeded = StatusTLE
const StatusRuntimeError = StatusRE
const StatusCompileError = StatusCE
const StatusJudgeError = StatusSE

type TestCaseResult struct {
	// Index is the 1-based position of the case in the order the judge ran it.
	// It is the number a verdict names ("Wrong Answer on Test Case 3"): CaseName
	// is the testdata filename, which is opaque to a contestant and does not
	// survive a reorder of the case list.
	Index    int              `json:"index,omitempty"`
	CaseName string           `json:"case_name"`
	Status   SubmissionStatus `json:"status"`
	Time     int              `json:"time"`
	Memory   int              `json:"memory"`
	Score    int              `json:"score"`
	Detail   string           `json:"detail,omitempty"`
}

type Submission struct {
	ID             string           `json:"id"`
	ProblemID      string           `json:"problem_id"`
	UserID         string           `json:"user_id"`
	Username       string           `json:"username,omitempty"`
	ContestID      string           `json:"contest_id,omitempty"`
	Language       string           `json:"language"`
	SourceCode     string           `json:"source_code,omitempty"`
	SubmissionType string           `json:"submission_type"` // "code" or "output"
	CodeSize       int              `json:"code_size"`
	Status         SubmissionStatus `json:"status"`
	Score          int              `json:"score"`
	TimeUsed       int              `json:"time_used"`
	MemoryUsed     int              `json:"memory_used"`
	CompileOutput  string           `json:"compile_output,omitempty"`
	// JudgeError is the judge's own failure text for a System Error verdict.
	// Only admins and the problem's setters receive it; the contestant sees the
	// generic "the judge failed" message.
	JudgeError  string           `json:"judge_error,omitempty"`
	JudgeResult []TestCaseResult `json:"judge_result,omitempty"`
	JudgedBy    string           `json:"judged_by"`
	RemoteID    string           `json:"remote_id,omitempty"`
	RemoteURL   string           `json:"remote_url,omitempty"`
	BotID       string           `json:"bot_id,omitempty"`
	BotSlug     string           `json:"bot_slug,omitempty"`
	IsRemote    bool             `json:"is_remote"`
	RemoteOJ    string           `json:"remote_oj,omitempty"`
	IsUpsolving bool             `json:"is_upsolving"`
	// ProblemSlug/ProblemTitle are filled by listings that join problems, so a
	// status table can name the problem without a second lookup per row.
	ProblemSlug  string `json:"problem_slug,omitempty"`
	ProblemTitle string `json:"problem_title,omitempty"`
	// ProblemIndex is the contest letter (A, B, …), which is how a round is read
	// and the only way to identify a problem inside one. Filled by the
	// contest-scoped listings; empty everywhere else, since a practice problem
	// has no letter.
	ProblemIndex string `json:"problem_index,omitempty"`
	// CanView / CanRejudge are per-viewer capabilities computed by the handler;
	// they are never stored.
	CanView    bool       `json:"can_view,omitempty"`
	CanRejudge bool       `json:"can_rejudge,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	JudgedAt   *time.Time `json:"judged_at,omitempty"`
}

type SubmitRequest struct {
	ProblemID      string `json:"problem_id"`
	Language       string `json:"language"`
	SourceCode     string `json:"source_code"`
	ContestID      string `json:"contest_id,omitempty"`
	SubmissionType string `json:"submission_type,omitempty"`
}

type PendingRemoteSubmission struct {
	ID       string `json:"id"`
	RemoteID string `json:"remote_id"`
	BotID    string `json:"bot_id"`
	BotSlug  string `json:"bot_slug"`
	Status   string `json:"status"`
	Platform string `json:"platform"`
}

type SubmissionFilter struct {
	UserID    string
	ProblemID string
	Language  string
	Status    string
	// ContestID, Username and IDs are honoured by the problem-scoped listings
	// (ListByProblemManage, ListIDsForProblemRejudge); ListByContest ignores them.
	ContestID string
	Username  string
	IDs       []string
}

// PublicSubmissionFilter drives the site-wide status page.
type PublicSubmissionFilter struct {
	SubmissionFilter
	ProblemSlug string
	// ViewerID is always allowed to see their own rows. ShowAll (admins) lifts
	// the hidden-problem and running-contest restrictions entirely.
	ViewerID string
	ShowAll  bool
}
