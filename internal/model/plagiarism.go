package model

import "time"

type PlagiarismReportStatus string

const (
	PlagiarismStatusPending   PlagiarismReportStatus = "pending"
	PlagiarismStatusRunning   PlagiarismReportStatus = "running"
	PlagiarismStatusCompleted PlagiarismReportStatus = "completed"
	PlagiarismStatusFailed    PlagiarismReportStatus = "failed"
)

type PlagiarismPairStatus string

const (
	PlagiarismPairPending PlagiarismPairStatus = "pending"
	PlagiarismPairFlagged PlagiarismPairStatus = "flagged"
	PlagiarismPairIgnored PlagiarismPairStatus = "ignored"
)

type PlagiarismReport struct {
	ID           string                 `json:"id"`
	ContestID    string                 `json:"contest_id"`
	Status       PlagiarismReportStatus `json:"status"`
	Threshold    float64                `json:"threshold"`
	TotalPairs   int                    `json:"total_pairs"`
	FlaggedCount int                    `json:"flagged_count"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	CreatedBy    string                 `json:"created_by"`
	CreatedAt    time.Time              `json:"created_at"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
}

type PlagiarismPair struct {
	ID            string               `json:"id"`
	ReportID      string               `json:"report_id"`
	ProblemID     string               `json:"problem_id"`
	SubmissionAID string               `json:"submission_a_id"`
	SubmissionBID string               `json:"submission_b_id"`
	UserAID       string               `json:"user_a_id"`
	UserBID       string               `json:"user_b_id"`
	Similarity    float64              `json:"similarity"`
	Status        PlagiarismPairStatus `json:"status"`
	MatchedLines  int                  `json:"matched_lines"`
	CreatedAt     time.Time            `json:"created_at"`
	UpdatedAt     time.Time            `json:"updated_at"`
}

type PlagiarismPairDetail struct {
	PlagiarismPair
	ProblemTitle    string `json:"problem_title"`
	UserAUsername   string `json:"user_a_username"`
	UserBUsername   string `json:"user_b_username"`
	SubmissionALang string `json:"submission_a_lang"`
	SubmissionBLang string `json:"submission_b_lang"`
}

type PlagiarismReportDetail struct {
	PlagiarismReport
	ContestTitle string                 `json:"contest_title"`
	Pairs        []PlagiarismPairDetail `json:"pairs"`
}

type PlagiarismCheckRequest struct {
	ContestID string  `json:"contest_id"`
	Threshold float64 `json:"threshold,omitempty"`
}

type PlagiarismPairUpdateRequest struct {
	Status PlagiarismPairStatus `json:"status"`
}
