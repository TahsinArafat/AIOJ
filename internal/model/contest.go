package model

import (
	"encoding/json"
	"time"
)

type Contest struct {
	ID                    string                  `json:"id"`
	DisplayID             int                     `json:"display_id"`
	Slug                  string                  `json:"slug,omitempty"`
	Title                 string                  `json:"title"`
	Type                  string                  `json:"type"`
	Format                string                  `json:"format"`
	FormatConfig          json.RawMessage         `json:"format_config,omitempty"`
	StartTime             time.Time               `json:"start_time"`
	EndTime               time.Time               `json:"end_time"`
	FreezeTime            *time.Time              `json:"freeze_time,omitempty"`
	Password              string                  `json:"-"`
	HasPassword           bool                    `json:"has_password"`
	Visible               bool                    `json:"visible"`
	Description           string                  `json:"description,omitempty"`
	RegistrationRequired  bool                    `json:"registration_required"`
	RegistrationDeadline  *time.Time              `json:"registration_deadline,omitempty"`
	MaxParticipants       *int                    `json:"max_participants,omitempty"`
	Division              int                     `json:"division"`
	EducationalConfig     *EducationalRoundConfig `json:"educational_config,omitempty"`
	HackPhaseEnabled      bool                    `json:"hack_phase_enabled"`
	HackPhaseStart        *time.Time              `json:"hack_phase_start,omitempty"`
	HackPhaseEnd          *time.Time              `json:"hack_phase_end,omitempty"`
	TeamSize              int                     `json:"team_size"`
	IsTeamContest         bool                    `json:"is_team_contest"`
	UpsolvingEnabled      bool                    `json:"upsolving_enabled"`
	VirtualContestEnabled bool                    `json:"virtual_contest_enabled"`
	PDFEnabled            bool                    `json:"pdf_enabled"`
	StatementHidden       bool                    `json:"statement_hidden"`
	GroupID               string                  `json:"group_id,omitempty"`
	RatingCalculated      bool                    `json:"rating_calculated"`
	CreatedBy             string                  `json:"created_by"`
	CreatedAt             time.Time               `json:"created_at"`
	Status                string                  `json:"status"` // draft|pending|published|rejected
	ReviewNote            string                  `json:"review_note,omitempty"`
	// IsRated was in the schema since migration 000003 and no Go code ever read
	// or wrote it, so every contest was implicitly rated and an unrated round
	// could not exist. It is now part of the model and of the create/update path.
	IsRated     bool `json:"is_rated"`
	RatedForMin int  `json:"rated_for_min"`
	RatedForMax int  `json:"rated_for_max"`
}

// Contest review states. A setter proposes a round; an admin publishes it. Only
// a published contest is visible to competitors, and only a published, rated
// contest may have ratings applied.
const (
	ContestStatusDraft     = "draft"
	ContestStatusPending   = "pending"
	ContestStatusPublished = "published"
	ContestStatusRejected  = "rejected"
)

const (
	ContestPermissionManager = "manager"
	ContestPermissionJudge   = "judge"
	ContestPermissionTester  = "tester"
)

// MaxContestProblems bounds a round at the 26 single-letter indices the schema
// stores (contest_problems.index is CHAR(1)).
const MaxContestProblems = 26

// ContestProblemIndex is the single place a round's problem letter is derived.
//
// The letter is a function of position in sort_order, never a stored fact that
// can drift: removing a problem closes the gap and re-letters what follows, so
// every surface (problems tab, standings, booklet, PDF) reads the same letter
// for the same problem. Before this there were four independent computations and
// a removal left the stored index stale, which is how a contest ended up showing
// "B" on one tab and "A" in the standings for the same problem.
func ContestProblemIndex(position int) string {
	if position < 0 {
		position = 0
	}
	if position >= MaxContestProblems {
		position = MaxContestProblems - 1
	}
	return string(rune('A' + position))
}

type ContestProblem struct {
	ContestID string `json:"contest_id"`
	ProblemID string `json:"problem_id"`
	Index     string `json:"index"`
	Title     string `json:"title,omitempty"`
	Slug      string `json:"slug,omitempty"`
	Score     int    `json:"score"`
	SortOrder int    `json:"sort_order"`
	// Per-viewer progress, filled only by the contest detail endpoint. The
	// problem list shows a solved tick and the solve count for the round, which
	// a competitor needs to decide what to open next — the list used to render a
	// bare "Solve" button and nothing else.
	//
	// Omitted from the payload for anonymous callers rather than sent as false,
	// so "not solved" and "we do not know who you are" stay distinguishable.
	SolvedCount   int  `json:"solved_count,omitempty"`
	MySolved      bool `json:"my_solved,omitempty"`
	MyAttempted   bool `json:"my_attempted,omitempty"`
	MySubmissions int  `json:"my_submissions,omitempty"`
}

// ContestProblemProgress is one problem's solve state: how many participants in
// the round have solved it, and what the viewing user has done with it.
type ContestProblemProgress struct {
	SolvedCount   int
	MySolved      bool
	MyAttempted   bool
	MySubmissions int
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

// ContestDisplayName is what a standings row should be labelled.
//
// A generated onsite account is a users row whose username is a throwaway
// handle ("team_1a2b3c4d"); the competitor knows themselves as the team name
// entered in the Onsite Teams panel. That name lives in onsite_batch_users,
// keyed by (contest_id, used_by), so labelling a scoreboard is a lookup across
// both tables. Empty fields mean "fall back to the username".
type ContestDisplayName struct {
	TeamName    string `json:"team_name,omitempty"`
	Institution string `json:"institution,omitempty"`
}

type CreateContestRequest struct {
	Title                 string          `json:"title"`
	Slug                  string          `json:"slug,omitempty"`
	Type                  string          `json:"type"`
	Format                string          `json:"format"`
	FormatConfig          json.RawMessage `json:"format_config,omitempty"`
	Division              int             `json:"division"`
	StartTime             time.Time       `json:"start_time"`
	EndTime               time.Time       `json:"end_time"`
	FreezeTime            *time.Time      `json:"freeze_time,omitempty"`
	Password              *string         `json:"password"`
	Description           string          `json:"description,omitempty"`
	Visible               *bool           `json:"visible,omitempty"`
	ProblemIDs            []string        `json:"problem_ids"`
	PDFEnabled            *bool           `json:"pdf_enabled,omitempty"`
	StatementHidden       *bool           `json:"statement_hidden,omitempty"`
	UpsolvingEnabled      *bool           `json:"upsolving_enabled,omitempty"`
	VirtualContestEnabled *bool           `json:"virtual_contest_enabled,omitempty"`
	GroupID               string          `json:"group_id,omitempty"`
	// Registration controls. Without these on the create request there was no
	// way to make a registration-required contest, so the registration
	// endpoints always answered 400 "registration not required for this
	// contest". model.Contest has always carried the fields; only the request
	// DTO was missing them.
	RegistrationRequired bool       `json:"registration_required"`
	RegistrationDeadline *time.Time `json:"registration_deadline,omitempty"`
	MaxParticipants      *int       `json:"max_participants,omitempty"`
	// Rating controls. is_rated has existed in the schema since migration 000003
	// but was absent from every Go struct, so a contest could not be declared
	// unrated. RatedForMin/Max bound which competitors the round counts for.
	// Pointers because this DTO backs both create and update: on update an
	// omitted field must mean "leave it alone", which a zero-valued bool cannot
	// express (false is a real choice — an unrated round).
	IsRated     *bool `json:"is_rated,omitempty"`
	RatedForMin *int  `json:"rated_for_min,omitempty"`
	RatedForMax *int  `json:"rated_for_max,omitempty"`
}

type ContestRegistration struct {
	ContestID    string    `json:"contest_id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username,omitempty"`
	RegisteredAt time.Time `json:"registered_at"`
}

type TeamRegistration struct {
	ID           string    `json:"id"`
	ContestID    string    `json:"contest_id"`
	TeamID       string    `json:"team_id"`
	TeamName     string    `json:"team_name,omitempty"`
	RegisteredAt time.Time `json:"registered_at"`
}

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
