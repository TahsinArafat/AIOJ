package generate

type BriefPreset struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Values      Request `json:"values"`
}

func BriefPresets() []BriefPreset {
	return []BriefPreset{
		{ID: "solo-cf", Name: "Solo CF practice", Description: "Individual online practice with Codeforces-style calibration.", Values: Request{SolverMode: SolverSolo, EventContext: EventPractice, FormatProfile: ProfileCodeforces, ExpectedSolveMinutes: 45, InsightEmphasis: "balanced"}},
		{ID: "codeforces-round", Name: "Codeforces round", Description: "Individual timed online round problem.", Values: Request{SolverMode: SolverSolo, EventContext: EventOnline, FormatProfile: ProfileCodeforces, ExpectedSolveMinutes: 35, InsightEmphasis: "insight"}},
		{ID: "icpc-onsite", Name: "ICPC onsite team", Description: "Three-person onsite contest problem with no subtasks.", Values: Request{SolverMode: SolverTeam, TeamSize: 3, EventContext: EventOnsite, FormatProfile: ProfileICPC, ExpectedSolveMinutes: 60, InsightEmphasis: "balanced"}},
		{ID: "oi-batch", Name: "OI-style batch", Description: "Individual batch task with optional partial scoring.", Values: Request{SolverMode: SolverSolo, EventContext: EventOnsite, FormatProfile: ProfileOI, ExpectedSolveMinutes: 120, InsightEmphasis: "insight", HasSubtasks: true}},
	}
}

func profileInstructions(r Request) string {
	switch r.FormatProfile {
	case ProfileICPC:
		return "ICPC onsite profile: team of three, penalty-sensitive implementation, clear batch I/O, no subtasks, concise editorial."
	case ProfileCodeforces:
		return "Codeforces profile: solo-oriented, concise statement, adversarial edge cases and calibrated insight."
	case ProfileOI:
		return "OI-style standard batch profile: individual solver, optional subtasks, strong proof and constraints."
	default:
		return "General standard batch problem profile."
	}
}
