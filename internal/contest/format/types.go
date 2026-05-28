package format

import (
	"encoding/json"
	"time"
)

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
