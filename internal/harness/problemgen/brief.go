package problemgen

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
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

// renderIntent is the operator's request as a bullet list, without a preamble.
//
// Only the operator's intent is passed through. The pipeline owns everything
// that must be true of the output (tiers, determinism, limits), so the brief
// cannot weaken the gates by asking for them to be skipped.
//
// It renders only the fields the artifact can use. Every field was rendered for
// every artifact before, so an editorial — which takes no topics, rating or test
// count — was sent a brief reading "Topics: dp, Target rating: 1400, Target
// test-set size: about 10 cases", instructing the model to build a problem
// around inputs the setter had never been asked for and the artifact does not
// produce.
//
// Split out from the message as a whole so the same intent can be embedded in a
// larger one (see RenderFor) without carrying a header that contradicts it.
func (b Brief) renderIntent(artifact string) string {
	spec, ok := model.LookupArtifact(artifact)
	if !ok {
		// An unknown artifact is a caller error, but a brief that renders is
		// still more useful than a panic or an empty prompt; the widest scope
		// matches what ArtifactFields would fall back to.
		spec, _ = model.LookupArtifact(string(model.ArtifactFullProblem))
	}

	var sb strings.Builder
	if spec.Accepts(model.FieldTopics) && len(b.Tags) > 0 {
		fmt.Fprintf(&sb, "- Topics: %s\n", strings.Join(b.Tags, ", "))
	}
	if spec.Accepts(model.FieldRating) && b.Rating > 0 {
		fmt.Fprintf(&sb, "- Target rating: %d (Codeforces scale)\n", b.Rating)
	}
	if spec.Accepts(model.FieldRatingRange) && (b.RatingMin > 0 || b.RatingMax > 0) {
		fmt.Fprintf(&sb, "- Acceptable rating range: %d-%d\n", b.RatingMin, b.RatingMax)
	}
	if spec.Accepts(model.FieldFormatProfile) && b.FormatProfile != "" {
		fmt.Fprintf(&sb, "- Format profile: %s\n", b.FormatProfile)
	}
	if spec.Accepts(model.FieldSolverMode) && b.SolverMode != "" {
		fmt.Fprintf(&sb, "- Solver mode: %s", b.SolverMode)
		if b.TeamSize > 0 {
			fmt.Fprintf(&sb, " (team of %d)", b.TeamSize)
		}
		sb.WriteString("\n")
	}
	if spec.Accepts(model.FieldEventContext) && b.EventContext != "" {
		fmt.Fprintf(&sb, "- Event context: %s\n", b.EventContext)
	}
	if spec.Accepts(model.FieldAudienceLevel) && b.AudienceLevel != "" {
		fmt.Fprintf(&sb, "- Audience: %s\n", b.AudienceLevel)
	}
	if spec.Accepts(model.FieldExpectedSolveTime) && b.ExpectedSolveMinutes > 0 {
		fmt.Fprintf(&sb, "- Expected solve time: %d minutes\n", b.ExpectedSolveMinutes)
	}
	if spec.Accepts(model.FieldPrimaryTopics) && len(b.PrimaryTopics) > 0 {
		fmt.Fprintf(&sb, "- Primary topics: %s\n", strings.Join(b.PrimaryTopics, ", "))
	}
	if spec.Accepts(model.FieldSecondaryTopics) && len(b.SecondaryTopics) > 0 {
		fmt.Fprintf(&sb, "- Secondary topics: %s\n", strings.Join(b.SecondaryTopics, ", "))
	}
	if spec.Accepts(model.FieldRequiredTechniques) && len(b.RequiredTechniques) > 0 {
		fmt.Fprintf(&sb, "- Must require: %s\n", strings.Join(b.RequiredTechniques, ", "))
	}
	if spec.Accepts(model.FieldForbiddenTechniques) && len(b.ForbiddenTechniques) > 0 {
		fmt.Fprintf(&sb, "- Must not require: %s\n", strings.Join(b.ForbiddenTechniques, ", "))
	}
	if spec.Accepts(model.FieldInsightEmphasis) && b.InsightEmphasis != "" {
		fmt.Fprintf(&sb, "- Emphasis: %s\n", b.InsightEmphasis)
	}
	if spec.Accepts(model.FieldStoryTone) && b.StoryTone != "" {
		fmt.Fprintf(&sb, "- Story tone: %s\n", b.StoryTone)
	}
	if spec.Accepts(model.FieldConstraintGuidance) && b.ConstraintGuidance != "" {
		fmt.Fprintf(&sb, "- Constraint guidance: %s\n", b.ConstraintGuidance)
	}
	// TestCaseCount was collected by the brief form, validated to 1-100, and
	// then never rendered — the harness silently ignored the requested size.
	// The matrix is tiers x shapes x seeds_per_cell, so state both the target
	// and how to reach it.
	if spec.Accepts(model.FieldTestCaseCount) && b.TestCaseCount > 0 {
		fmt.Fprintf(&sb, "- Target test-set size: about %d cases. materialize_matrix produces one case per "+
			"(tier, shape) cell, so set seeds_per_cell to roughly %d to reach this count.\n",
			b.TestCaseCount, seedsPerCellFor(b.TestCaseCount))
	}
	if spec.Accepts(model.FieldHasSubtasks) && b.HasSubtasks {
		sb.WriteString("- Subtask scoring with partial credit.\n")
	}
	if spec.Accepts(model.FieldAdditionalPrompt) && b.AdditionalPrompt != "" {
		fmt.Fprintf(&sb, "- Additional guidance: %s\n", b.AdditionalPrompt)
	}
	return sb.String()
}

// seedsPerCellFor sizes seeds_per_cell against the five matrix tiers, assuming a
// conservative four shapes. The matrix is 5 x shapes cells, so this is an
// estimate the model adjusts once it has chosen its shapes.
func seedsPerCellFor(cases int) int {
	seeds := cases / (len(TierOrder) * 4)
	if seeds < 1 {
		return 1
	}
	if seeds > 10 {
		return 10
	}
	return seeds
}
