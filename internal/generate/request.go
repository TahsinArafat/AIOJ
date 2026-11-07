package generate

import "fmt"

// Request specifies what kind of problem to generate.
type Request struct {
	// Tags must contain at least one algorithmic topic (e.g. "dp", "graphs").
	Tags []string `json:"tags"`
	// Rating is a Codeforces-style difficulty rating in [800, 3500].
	Rating int `json:"rating"`
	// ProblemType is one of: "standard", "interactive", "output_only".
	ProblemType string `json:"problem_type"`
	// TestCaseCount is the desired number of test cases in [1, 100].
	TestCaseCount int `json:"testcase_count"`
	// HasSubtasks controls whether partial subtask scoring is requested.
	HasSubtasks bool `json:"has_subtasks"`
	// AdditionalPrompt is a free-form hint for the model (e.g. "grid DP on N×M array").
	AdditionalPrompt     string   `json:"additional_prompt,omitempty"`
	PrimaryTopics        []string `json:"primary_topics,omitempty"`
	SecondaryTopics      []string `json:"secondary_topics,omitempty"`
	RequiredTechniques   []string `json:"required_techniques,omitempty"`
	ForbiddenTechniques  []string `json:"forbidden_techniques,omitempty"`
	RatingMin            int      `json:"rating_min,omitempty"`
	RatingMax            int      `json:"rating_max,omitempty"`
	SolverMode           string   `json:"solver_mode,omitempty"`
	TeamSize             int      `json:"team_size,omitempty"`
	EventContext         string   `json:"event_context,omitempty"`
	FormatProfile        string   `json:"format_profile,omitempty"`
	ExpectedSolveMinutes int      `json:"expected_solve_minutes,omitempty"`
	AudienceLevel        string   `json:"audience_level,omitempty"`
	InsightEmphasis      string   `json:"insight_emphasis,omitempty"`
	StoryTone            string   `json:"story_tone,omitempty"`
	ConstraintGuidance   string   `json:"constraint_guidance,omitempty"`
}

// Accepted solver modes, event contexts, and format profiles.
const (
	SolverSolo = "solo"
	SolverTeam = "team"

	EventPractice = "practice"
	EventOnline   = "online"
	EventOnsite   = "onsite"

	ProfileGeneral    = "general"
	ProfileCodeforces = "codeforces"
	ProfileICPC       = "icpc"
	ProfileOI         = "oi"
)

// Normalize validates the request and fills defaults, returning a mutated
// copy. It is the single source of truth for brief rules so the HTTP handler
// and any future caller (the jobs API) share one implementation.
//
// Cross-field rules enforced here:
//   - the target rating must fall inside [rating_min, rating_max] and the
//     whole range must sit within the Codeforces 800–3500 scale;
//   - the ICPC profile is standard batch and therefore rejects subtasks;
//   - a team solver defaults to a team of three when the size is unspecified.
func (r Request) Normalize() (Request, error) {
	if len(r.Tags) == 0 {
		return r, fmt.Errorf("at least one tag is required")
	}
	if r.Rating < 800 || r.Rating > 3500 {
		return r, fmt.Errorf("rating must be between 800 and 3500")
	}
	if r.TestCaseCount < 1 || r.TestCaseCount > 100 {
		return r, fmt.Errorf("testcase_count must be between 1 and 100")
	}
	if r.ProblemType == "" {
		r.ProblemType = "standard"
	}
	if r.ProblemType != "standard" {
		return r, fmt.Errorf("v1 supports standard batch problems only")
	}

	if r.RatingMin == 0 {
		r.RatingMin = r.Rating - 100
	}
	if r.RatingMax == 0 {
		r.RatingMax = r.Rating + 100
	}
	if r.RatingMin < 800 || r.RatingMax > 3500 || r.RatingMin > r.Rating || r.RatingMax < r.Rating {
		return r, fmt.Errorf("rating range must contain rating and stay between 800 and 3500")
	}

	if r.SolverMode == "" {
		r.SolverMode = SolverSolo
	}
	if r.SolverMode != SolverSolo && r.SolverMode != SolverTeam {
		return r, fmt.Errorf("solver_mode must be solo or team")
	}

	if r.EventContext == "" {
		r.EventContext = EventPractice
	}
	if r.EventContext != EventPractice && r.EventContext != EventOnline && r.EventContext != EventOnsite {
		return r, fmt.Errorf("event_context must be practice, online, or onsite")
	}

	if r.FormatProfile == "" {
		r.FormatProfile = ProfileGeneral
	}
	switch r.FormatProfile {
	case ProfileGeneral, ProfileCodeforces, ProfileICPC, ProfileOI:
	default:
		return r, fmt.Errorf("unsupported format_profile")
	}
	if r.FormatProfile == ProfileICPC && r.HasSubtasks {
		return r, fmt.Errorf("ICPC profile does not support subtasks")
	}
	if r.FormatProfile == ProfileICPC {
		r.SolverMode = SolverTeam
		r.TeamSize = 3
		r.EventContext = EventOnsite
	}
	if r.FormatProfile == ProfileCodeforces {
		r.SolverMode = SolverSolo
	}
	if r.FormatProfile == ProfileOI {
		r.SolverMode = SolverSolo
	}

	if r.SolverMode == SolverTeam && r.TeamSize == 0 {
		r.TeamSize = 3
	}
	if r.TeamSize < 0 || r.TeamSize > 8 {
		return r, fmt.Errorf("team_size must be between 1 and 8")
	}

	if r.ExpectedSolveMinutes < 0 || r.ExpectedSolveMinutes > 360 {
		return r, fmt.Errorf("expected_solve_minutes must be between 1 and 360")
	}
	if r.ExpectedSolveMinutes == 0 {
		if r.FormatProfile == ProfileICPC {
			r.ExpectedSolveMinutes = 60
		} else {
			r.ExpectedSolveMinutes = 45
		}
	}

	return r, nil
}
