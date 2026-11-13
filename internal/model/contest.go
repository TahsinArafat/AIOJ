package model

import (
	"encoding/json"
	"time"
)

type Contest struct {
	ID           string          `json:"id"`
	DisplayID    int             `json:"display_id"`
	Slug         string          `json:"slug,omitempty"`
	Title        string          `json:"title"`
	Type         string          `json:"type"`
	Format       string          `json:"format"`
	FormatConfig json.RawMessage `json:"format_config,omitempty"`
	StartTime    time.Time       `json:"start_time"`
	EndTime      time.Time       `json:"end_time"`
	FreezeTime   *time.Time      `json:"freeze_time,omitempty"`
	// PasswordHash is the bcrypt hash of the round's access password. It never
	// leaves the process: JSON redacts it and no handler ever assigns it onto a
	// response. The column used to be a plaintext `password` that nothing
	// compared against, which is why "password protected" contests were public.
	PasswordHash string `json:"-"`
	// HasPassword is the only signal the client gets, and it needs it: the
	// contest page renders an unlock prompt when this is set and the caller has
	// no grant yet.
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
	// AccessConfig is the per-surface answer to "who may see this part of the
	// round". It replaces the single ScoreboardVisibility field, which was the
	// first surface to need its own answer and turned out not to be the last.
	//
	// A round is not one audience. An onsite final keeps its statements in the
	// room and puts its board on a projector; a hiring round may publish the
	// problem list as a teaser and hold the statements back; a school round may
	// hand out a printable booklet while the statements stay closed in the
	// browser. Each of those is a real combination and none of them can be
	// expressed by one boolean.
	//
	// Stored as JSONB because the set of surfaces grows and a column per surface
	// would mean a migration per surface, with the default for a new surface
	// having to be decided in SQL every time. Absent key means "inherit", so an
	// entry is only ever a deliberate override.
	AccessConfig     AccessConfig `json:"access_config"`
	GroupID          string       `json:"group_id,omitempty"`
	RatingCalculated bool         `json:"rating_calculated"`
	CreatedBy        string       `json:"created_by"`
	CreatedAt        time.Time    `json:"created_at"`
	Status           string       `json:"status"` // draft|pending|published|rejected
	ReviewNote       string       `json:"review_note,omitempty"`
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

// AccessAudience is who may reach one surface of a round. Deliberately a small
// vocabulary rather than a bool or an enum per surface: a bool cannot grow a
// third state without a migration that rewrites every row's meaning (the reason
// scoreboard_visibility was a string in migration 000088), and one enum per
// surface would give the same word different spellings on each.
const (
	// AccessParticipants: reaching this surface is part of reading the round, so
	// the password and group rules apply exactly as they do to the statements.
	// This is the default for every surface, and it is what every round did
	// before any of these settings existed.
	AccessParticipants = "participants"
	// AccessPublic: anyone may reach this surface, including a signed-out visitor
	// who never held the password. The round's OTHER surfaces are unaffected —
	// this is per surface, which is the whole point.
	AccessPublic = "public"
	// AccessStaff: only the people running the round. For a surface that is an
	// organizer tool rather than a part of the competition (the printable
	// booklet, the per-problem statistics), this is the honest setting: it says
	// "this is not for competitors" instead of making an organizer pick an
	// audience that happens to include them.
	AccessStaff = "staff"
)

// AccessSurfaces is the round's gated surfaces, in the order the manage screen
// lists them. The set is closed on purpose: surface keys are validated against
// it, so a typo in a settings write is a 400 rather than a config entry nothing
// reads, and adding a surface is one line here plus one gate call site.
const (
	// SurfaceParticipate is the round itself — may this caller take part at all.
	// Every other surface inherits it when it has no override of its own, which
	// is what makes an unset AccessConfig mean "exactly today's behaviour": the
	// base gate is the password and group rules, and every surface starts there.
	SurfaceParticipate = "participate"
	// SurfaceProblemList is the round's problem list: letters, titles and the
	// per-problem solve counts. Separable from the statements because a teaser
	// round publishes what it will ask without publishing what it asks.
	SurfaceProblemList = "problem_list"
	// SurfaceStatement is one problem's full statement.
	SurfaceStatement = "statement"
	// SurfacePDF is the printable booklet. Has its own answer because printing is
	// a different act from browsing: a round may hand out a booklet at the door
	// while keeping the statements closed in the browser.
	SurfacePDF = "pdf"
	// SurfaceSubmit is the right to send a solution.
	SurfaceSubmit = "submit"
	// SurfaceStandings is the scoreboard and the per-problem solve counts.
	SurfaceStandings = "standings"
)

// AccessSurfaceKeys is every valid surface key. Used by validation and by the UI
// to render the table, so the two cannot disagree about what exists.
var AccessSurfaceKeys = []string{
	SurfaceParticipate,
	SurfaceProblemList,
	SurfaceStatement,
	SurfacePDF,
	SurfaceSubmit,
	SurfaceStandings,
}

// ValidAccessSurface reports whether key names a real surface.
func ValidAccessSurface(key string) bool {
	for _, k := range AccessSurfaceKeys {
		if k == key {
			return true
		}
	}
	return false
}

// ValidAccessAudience rejects anything outside the allowed set. Validated in the
// handler rather than left to a constraint, so a bad value is a 400 with a
// reason instead of a 500.
func ValidAccessAudience(a string) bool {
	return a == AccessParticipants || a == AccessPublic || a == AccessStaff
}

// AccessConfig maps a surface key to the audience that may reach it. An absent
// key means "inherit", which for every surface is SurfaceParticipate.
//
// Inheritance rather than a stored default per surface is what lets this ship
// without a backfill that has to guess: a round created before this existed has
// an empty map, every surface resolves through SurfaceParticipate, and the
// result is byte-for-byte the behaviour the round had yesterday.
type AccessConfig map[string]string

// Audience resolves one surface to its effective audience.
//
// The two rules, in order:
//
//  1. An explicit entry wins.
//  2. Otherwise the surface inherits SurfaceParticipate, and if that is also
//     unset, participants.
//
// Note what is NOT inherited: a surface set to "public" does not make the
// surfaces below it public, and SurfaceParticipate set to "public" does not
// override an explicit "staff" on some other surface. Inheritance flows one way
// only — an unset surface follows the base, never the other way around — because
// the alternative (a public surface widening its neighbours) is how "I published
// the scoreboard" would silently publish the statements, which is the exact bug
// the scoreboard axis was introduced to fix.
func (a AccessConfig) Audience(surface string) string {
	if v, ok := a[surface]; ok && ValidAccessAudience(v) {
		return v
	}
	if v, ok := a[SurfaceParticipate]; ok && ValidAccessAudience(v) {
		return v
	}
	return AccessParticipants
}

// NormalizeAccessConfig returns a config with every invalid entry dropped and
// every empty map turned into a non-nil one.
//
// Dropped rather than rejected, because this runs at the store boundary where
// there is no response to write: a row written by an older build, or by hand,
// must not be able to make a surface permanently unreachable. Validation with a
// 400 happens in the handler, on input a user supplied.
func NormalizeAccessConfig(a AccessConfig) AccessConfig {
	out := AccessConfig{}
	for k, v := range a {
		if ValidAccessSurface(k) && ValidAccessAudience(v) {
			out[k] = v
		}
	}
	return out
}

// PublicSurfaces lists the surfaces a caller who cannot read the round may still
// reach, in the canonical surface order so the message is stable.
//
// Used by the refusal body so a spectator holding a link is told what they can
// follow it to. Without this the only honest thing the server could say was "this
// round is private", which reads as "you were sent a dead link" on a round whose
// board is on a projector.
func (c *Contest) PublicSurfaces() []string {
	var out []string
	for _, s := range AccessSurfaceKeys {
		if s == SurfaceParticipate {
			// Not a surface a stranger is ever told about: it is the round's base
			// gate, so "participate is public" already means they can read
			// everything and would not have reached this refusal at all.
			continue
		}
		if c.AccessConfig.Audience(s) == AccessPublic {
			out = append(out, s)
		}
	}
	return out
}

// IsScoreboardPublic is kept as a named question because the scoreboard asks it
// in several places — the board itself, the per-problem solve counts, and the
// refusal body that tells a spectator the board is reachable — and a call site
// that spelled the map lookup itself is how the three would drift.
func (c *Contest) IsScoreboardPublic() bool {
	return c.AccessConfig.Audience(SurfaceStandings) == AccessPublic
}

// LegacyScoreboardVisibility translates the pre-access_config column value into
// the new map. Used by the migration's own backfill logic and by any test that
// wants to express "this round had a public board" in the old spelling.
func LegacyScoreboardVisibility(v string) AccessConfig {
	a := AccessConfig{}
	if v == AccessPublic {
		a[SurfaceStandings] = AccessPublic
	}
	return a
}

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

// CreateContestRequest backs both create and update. Fields that are a real
// choice at their zero value are pointers, so an omitted field on update means
// "leave it alone" rather than silently resetting it: Division 0 is "Open",
// RegistrationRequired false is "registration off", and IsRated false is an
// unrated round.
type CreateContestRequest struct {
	Title           string          `json:"title"`
	Slug            string          `json:"slug,omitempty"`
	Type            string          `json:"type"`
	Format          string          `json:"format"`
	FormatConfig    json.RawMessage `json:"format_config,omitempty"`
	Division        *int            `json:"division,omitempty"`
	StartTime       time.Time       `json:"start_time"`
	EndTime         time.Time       `json:"end_time"`
	FreezeTime      *time.Time      `json:"freeze_time,omitempty"`
	Password        *string         `json:"password"`
	Description     string          `json:"description,omitempty"`
	Visible         *bool           `json:"visible,omitempty"`
	ProblemIDs      []string        `json:"problem_ids"`
	PDFEnabled      *bool           `json:"pdf_enabled,omitempty"`
	StatementHidden *bool           `json:"statement_hidden,omitempty"`
	// AccessConfig is the per-surface audience map. A whole-map replace rather
	// than a per-surface patch: the manage screen renders all surfaces at once and
	// sends all of them back, so merge semantics would only create a way for a
	// stale tab to resurrect a setting another tab had just cleared.
	//
	// A map rather than a pointer, because "send an empty map" and "send nothing"
	// are genuinely different requests and both are meaningful: the first means
	// "every surface back to inheriting the base", the second means "leave the
	// audiences alone while I rename the round". Both decode to a nil map from a
	// *map[string]string without presence tracking, so Update reads the key set —
	// the same technique the nullable fields needed, for the same reason.
	AccessConfig          AccessConfig `json:"access_config,omitempty"`
	UpsolvingEnabled      *bool        `json:"upsolving_enabled,omitempty"`
	VirtualContestEnabled *bool        `json:"virtual_contest_enabled,omitempty"`
	GroupID               string       `json:"group_id,omitempty"`
	// Registration controls. Without these on the create request there was no
	// way to make a registration-required contest, so the registration
	// endpoints always answered 400 "registration not required for this
	// contest". model.Contest has always carried the fields; only the request
	// DTO was missing them.
	RegistrationRequired *bool      `json:"registration_required,omitempty"`
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
