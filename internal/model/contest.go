package model

import "time"

type Contest struct {
	ID                   string     `json:"id"`
	Title                string     `json:"title"`
	Type                 string     `json:"type"`
	StartTime            time.Time  `json:"start_time"`
	EndTime              time.Time  `json:"end_time"`
	FreezeTime           *time.Time `json:"freeze_time,omitempty"`
	Password             string     `json:"-"`
	Visible              bool       `json:"visible"`
	Description          string     `json:"description,omitempty"`
	RegistrationRequired bool       `json:"registration_required"`
	RegistrationDeadline *time.Time `json:"registration_deadline,omitempty"`
	MaxParticipants      *int       `json:"max_participants,omitempty"`
	Division             int        `json:"division"`
	EducationalConfig    *EducationalRoundConfig `json:"educational_config,omitempty"`
	HackPhaseEnabled     bool       `json:"hack_phase_enabled"`
	HackPhaseStart       *time.Time `json:"hack_phase_start,omitempty"`
	HackPhaseEnd         *time.Time `json:"hack_phase_end,omitempty"`
	CreatedBy            string     `json:"created_by"`
	CreatedAt            time.Time  `json:"created_at"`
}

type ContestProblem struct {
	ContestID string `json:"contest_id"`
	ProblemID string `json:"problem_id"`
	Index     string `json:"index"`
	Score     int    `json:"score"`
	SortOrder int    `json:"sort_order"`
}

type ProblemResult struct {
	Solved   bool `json:"solved"`
	Attempts int  `json:"attempts"`
	Time     int  `json:"time"`
	Score    int  `json:"score"`
	Pending  int  `json:"pending"`
}

type ContestRankEntry struct {
	UserID       string                   `json:"user_id"`
	Username     string                   `json:"username"`
	Problems     map[string]ProblemResult `json:"problems"`
	TotalSolved  int                      `json:"total_solved"`
	TotalPenalty int                      `json:"total_penalty"`
	TotalScore   int                      `json:"total_score"`
	LastACTime   *time.Time               `json:"last_ac_time,omitempty"`
}

type ContestPermission struct {
	ContestID   string `json:"contest_id"`
	UserID      string `json:"user_id"`
	Username    string `json:"username,omitempty"`
	AccessLevel string `json:"access_level"`
}

type CreateContestRequest struct {
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Division    int        `json:"division"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     time.Time  `json:"end_time"`
	FreezeTime  *time.Time `json:"freeze_time,omitempty"`
	Password    string     `json:"password,omitempty"`
	Description string     `json:"description,omitempty"`
	ProblemIDs  []string   `json:"problem_ids"`
}

type ContestRegistration struct {
	ContestID    string    `json:"contest_id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username,omitempty"`
	RegisteredAt time.Time `json:"registered_at"`
}

const (
	ContestTypeACM         = "acm"
	ContestTypeOI          = "oi"
	ContestTypeIOI         = "ioi"
	ContestTypePractice    = "practice"
	ContestTypeEducational = "educational"
)

const (
	DivisionNone = 0
	Division1    = 1
	Division2    = 2
	Division3    = 3
	Division4    = 4
)

var DivisionNames = map[int]string{
	DivisionNone: "Open",
	Division1:    "Div. 1",
	Division2:    "Div. 2",
	Division3:    "Div. 3",
	Division4:    "Div. 4",
}

func GetDivisionRange(division int) (min, max int) {
	switch division {
	case Division1:
		return 1900, 9999
	case Division2:
		return 0, 2099
	case Division3:
		return 0, 1599
	case Division4:
		return 0, 1399
	default:
		return 0, 9999
	}
}

func IsEligibleForDivision(division, rating int) bool {
	min, max := GetDivisionRange(division)
	return rating >= min && rating <= max
}

type EducationalRoundConfig struct {
	HackPhaseHours    int   `json:"hack_phase_hours"`
	ShowSolutions     bool  `json:"show_solutions"`
	AllowUpsolving    bool  `json:"allow_upsolving"`
	RatedForDivisions []int `json:"rated_for_divisions"`
}

func DefaultEducationalConfig() EducationalRoundConfig {
	return EducationalRoundConfig{
		HackPhaseHours:    24,
		ShowSolutions:     true,
		AllowUpsolving:    true,
		RatedForDivisions: []int{2, 3},
	}
}
