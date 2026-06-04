package model

import "time"

type SubmissionStatus string

const (
	StatusPending   SubmissionStatus = "pending"
	StatusJudging   SubmissionStatus = "judging"
	StatusRejudging SubmissionStatus = "rejudging"
	StatusAC        SubmissionStatus = "ac"
	StatusWA      SubmissionStatus = "wa"
	StatusTLE     SubmissionStatus = "tle"
	StatusMLE     SubmissionStatus = "mle"
	StatusRE      SubmissionStatus = "re"
	StatusCE      SubmissionStatus = "ce"
	StatusSE      SubmissionStatus = "se"
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
	Status        SubmissionStatus `json:"status"`
	Score         int              `json:"score"`
	TimeUsed      int              `json:"time_used"`
	MemoryUsed    int              `json:"memory_used"`
	CompileOutput string           `json:"compile_output,omitempty"`
	JudgeResult   []TestCaseResult `json:"judge_result,omitempty"`
	JudgedBy      string           `json:"judged_by"`
	RemoteID       string           `json:"remote_id,omitempty"`
	RemoteURL      string           `json:"remote_url,omitempty"`
	BotID          string           `json:"bot_id,omitempty"`
	BotSlug        string           `json:"bot_slug,omitempty"`
	IsRemote       bool             `json:"is_remote"`
	RemoteOJ       string           `json:"remote_oj,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	JudgedAt      *time.Time       `json:"judged_at,omitempty"`
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
}
