package problemgen

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Brief is the operator's request, as stored on the job.
//
// It mirrors generate.Request (kept separate so this package does not depend on
// the single-shot path) and is rendered to prose for the harness.
type Brief struct {
	Tags                 []string `json:"tags"`
	Rating               int      `json:"rating"`
	ProblemType          string   `json:"problem_type"`
	TestCaseCount        int      `json:"testcase_count"`
	HasSubtasks          bool     `json:"has_subtasks"`
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

// ParseBrief decodes a stored brief.
func ParseBrief(raw json.RawMessage) (Brief, error) {
	var b Brief
	if len(raw) == 0 {
		return b, nil
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return b, fmt.Errorf("problemgen: invalid brief: %w", err)
	}
	return b, nil
}

// Render turns a brief into the user message the harness receives.
//
// Only the operator's intent is passed through. The pipeline owns everything
// that must be true of the output (tiers, determinism, limits), so the brief
// cannot weaken the gates by asking for them to be skipped.
func (b Brief) Render() string {
	var sb strings.Builder
	sb.WriteString("Build one competitive-programming problem with these specifications:\n")
	if len(b.Tags) > 0 {
		fmt.Fprintf(&sb, "- Topics: %s\n", strings.Join(b.Tags, ", "))
	}
	if b.Rating > 0 {
		fmt.Fprintf(&sb, "- Target rating: %d (Codeforces scale)\n", b.Rating)
	}
	if b.RatingMin > 0 || b.RatingMax > 0 {
		fmt.Fprintf(&sb, "- Acceptable rating range: %d-%d\n", b.RatingMin, b.RatingMax)
	}
	if b.FormatProfile != "" {
		fmt.Fprintf(&sb, "- Format profile: %s\n", b.FormatProfile)
	}
	if b.SolverMode != "" {
		fmt.Fprintf(&sb, "- Solver mode: %s", b.SolverMode)
		if b.TeamSize > 0 {
			fmt.Fprintf(&sb, " (team of %d)", b.TeamSize)
		}
		sb.WriteString("\n")
	}
	if b.EventContext != "" {
		fmt.Fprintf(&sb, "- Event context: %s\n", b.EventContext)
	}
	if b.AudienceLevel != "" {
		fmt.Fprintf(&sb, "- Audience: %s\n", b.AudienceLevel)
	}
	if b.ExpectedSolveMinutes > 0 {
		fmt.Fprintf(&sb, "- Expected solve time: %d minutes\n", b.ExpectedSolveMinutes)
	}
	if len(b.PrimaryTopics) > 0 {
		fmt.Fprintf(&sb, "- Primary topics: %s\n", strings.Join(b.PrimaryTopics, ", "))
	}
	if len(b.SecondaryTopics) > 0 {
		fmt.Fprintf(&sb, "- Secondary topics: %s\n", strings.Join(b.SecondaryTopics, ", "))
	}
	if len(b.RequiredTechniques) > 0 {
		fmt.Fprintf(&sb, "- Must require: %s\n", strings.Join(b.RequiredTechniques, ", "))
	}
	if len(b.ForbiddenTechniques) > 0 {
		fmt.Fprintf(&sb, "- Must not require: %s\n", strings.Join(b.ForbiddenTechniques, ", "))
	}
	if b.InsightEmphasis != "" {
		fmt.Fprintf(&sb, "- Emphasis: %s\n", b.InsightEmphasis)
	}
	if b.StoryTone != "" {
		fmt.Fprintf(&sb, "- Story tone: %s\n", b.StoryTone)
	}
	if b.ConstraintGuidance != "" {
		fmt.Fprintf(&sb, "- Constraint guidance: %s\n", b.ConstraintGuidance)
	}
	if b.HasSubtasks {
		sb.WriteString("- Subtask scoring with partial credit.\n")
	}
	if b.AdditionalPrompt != "" {
		fmt.Fprintf(&sb, "- Additional guidance: %s\n", b.AdditionalPrompt)
	}
	sb.WriteString("\nCall the tools to build it. Nothing is published until a human reviews the result.")
	return sb.String()
}
