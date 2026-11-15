package model

import "sort"

// ArtifactType identifies what an AI generation job produces.
//
// A string-typed alias rather than a bare string: the set is closed, it crosses
// the wire, and store rows and job payloads both carry it, so an accidental typo
// should be a compile error at every call site that names one.
type ArtifactType string

const (
	ArtifactFullProblem ArtifactType = "full_problem"
	ArtifactStatement   ArtifactType = "statement"
	ArtifactTests       ArtifactType = "tests"
	ArtifactSolution    ArtifactType = "solution"
	ArtifactEditorial   ArtifactType = "editorial"
)

// BriefField identifies one input of an authoring brief.
//
// Fields are named as concepts, not as the JSON keys they land on, so the policy
// below can be read without holding the wire format in mind. Each one maps to a
// control in the brief form and to a line in the prompt the model is sent.
type BriefField string

const (
	FieldTopics              BriefField = "topics"
	FieldRating              BriefField = "rating"
	FieldRatingRange         BriefField = "rating_range"
	FieldFormatProfile       BriefField = "format_profile"
	FieldSolverMode          BriefField = "solver_mode"
	FieldEventContext        BriefField = "event_context"
	FieldInsightEmphasis     BriefField = "insight_emphasis"
	FieldExpectedSolveTime   BriefField = "expected_solve_minutes"
	FieldPrimaryTopics       BriefField = "primary_topics"
	FieldSecondaryTopics     BriefField = "secondary_topics"
	FieldRequiredTechniques  BriefField = "required_techniques"
	FieldForbiddenTechniques BriefField = "forbidden_techniques"
	FieldConstraintGuidance  BriefField = "constraint_guidance"
	FieldAudienceLevel       BriefField = "audience_level"
	FieldStoryTone           BriefField = "story_tone"
	FieldTestCaseCount       BriefField = "testcase_count"
	FieldHasSubtasks         BriefField = "has_subtasks"
	FieldAdditionalPrompt    BriefField = "additional_prompt"
)

// definitionFields DEFINE a problem, so only an artifact that invents one needs
// them: the full package. An artifact that rewrites part of an existing problem
// gets these from that problem instead — the worker passes it to the model as
// authoritative context. Requiring them again collected answers the run then
// ignored, which is what made a setter restate the topic and rating it was
// already being given. FieldTopics carries the same reasoning for the one field
// the server insists on.
var definitionFields = []BriefField{
	FieldTopics,
	FieldRating,
	FieldRatingRange,
	FieldFormatProfile,
	FieldSolverMode,
	FieldEventContext,
	FieldInsightEmphasis,
	FieldExpectedSolveTime,
	FieldPrimaryTopics,
}

// techiqueFields steer how a problem is solved. Anything that reasons about the
// solution — the problem itself, its tests, its reference solution — can use
// them; an editorial largely restates them from the solution it explains.
var techniqueFields = []BriefField{
	FieldSecondaryTopics,
	FieldRequiredTechniques,
	FieldForbiddenTechniques,
	FieldConstraintGuidance,
}

// testShapeFields describe the test suite, so only an artifact that produces one
// can use them.
var testShapeFields = []BriefField{FieldTestCaseCount, FieldHasSubtasks}

// voiceFields are register rather than content: how the prose is written, not
// what it is about. A statement and an editorial are the prose artifacts.
var voiceFields = []BriefField{FieldAudienceLevel, FieldStoryTone}

// ArtifactSpec is one artifact type: what it is called, whether it applies to an
// already-open problem, and the brief inputs it can use.
//
// It is serialised to the browser, so the JSON tags are part of the frontend
// contract (web/src/types/api.ts, AIArtifactSpec). The brief form renders its
// fields from this catalog over the wire, so the client cannot offer a control
// the server would reject or drop.
type ArtifactSpec struct {
	ID ArtifactType `json:"id"`
	// Label and Description are the picker's copy. They live here, beside the
	// policy, because the picker is rendered from this catalog — the alternative
	// is a second list in the frontend that drifts from this one.
	Label       string `json:"label"`
	Description string `json:"description"`
	// OfferedForExisting is false for the whole-package scope: regenerating every
	// artifact of a problem that already has them is not what "improve this
	// problem" means, so it is offered only when authoring a new one.
	OfferedForExisting bool `json:"offered_for_existing"`
	// Fields are the inputs this artifact accepts, in the order the form asks
	// for them. A field absent from this list is neither validated nor rendered
	// for this artifact.
	Fields []BriefField `json:"fields"`
}

// Accepts reports whether this artifact can use a field.
func (s ArtifactSpec) Accepts(f BriefField) bool {
	for _, have := range s.Fields {
		if have == f {
			return true
		}
	}
	return false
}

// AcceptsAny reports whether this artifact uses at least one of the fields.
//
// This is what lets a form section be composed rather than conditionally
// rendered: a section is shown when the artifact accepts anything in it, so
// adding a field never means editing a conditional per consumer.
func (s ArtifactSpec) AcceptsAny(fields ...BriefField) bool {
	for _, f := range fields {
		if s.Accepts(f) {
			return true
		}
	}
	return false
}

// Requires reports whether a field is mandatory for this artifact. Only topics
// are ever required, and only of the artifact that invents the problem.
func (s ArtifactSpec) Requires(f BriefField) bool {
	return f == FieldTopics && s.Accepts(FieldTopics)
}

// artifactCatalog is the single source of truth for the artifact scopes, in the
// order the picker shows them.
//
// Ordered by scope, widest first: the full package, then the pieces in the order
// a problem is built (statement, tests, solution), then the editorial written
// from them.
var artifactCatalog = []ArtifactSpec{
	{
		ID:                 ArtifactFullProblem,
		Label:              "Full package",
		Description:        "Statement, tests, generator & reference solution",
		OfferedForExisting: false,
		Fields:             join(definitionFields, techniqueFields, testShapeFields, voiceFields, []BriefField{FieldAdditionalPrompt}),
	},
	{
		ID:                 ArtifactStatement,
		Label:              "Statement & samples",
		Description:        "Rewrites the problem statement only",
		OfferedForExisting: true,
		Fields:             join(techniqueFields, voiceFields, []BriefField{FieldAdditionalPrompt}),
	},
	{
		ID:                 ArtifactTests,
		Label:              "Tests & generator",
		Description:        "Generates a new test suite and generator",
		OfferedForExisting: true,
		Fields:             join(testShapeFields, techniqueFields, []BriefField{FieldAdditionalPrompt}),
	},
	{
		ID:                 ArtifactSolution,
		Label:              "Reference solution",
		Description:        "Writes or replaces the reference solution",
		OfferedForExisting: true,
		Fields:             join(techniqueFields, []BriefField{FieldAdditionalPrompt}),
	},
	{
		ID:                 ArtifactEditorial,
		Label:              "Editorial",
		Description:        "Authors an editorial for this problem",
		OfferedForExisting: true,
		Fields:             join(voiceFields, []BriefField{FieldAdditionalPrompt}),
	},
}

// join concatenates field groups, dropping duplicates.
//
// The same field may be defensible in two groups (a technique list steers a
// statement's wording as well as its content), and a duplicate would render the
// control twice. Deduplication is done here so the groups above can be stated
// independently.
func join(groups ...[]BriefField) []BriefField {
	seen := make(map[BriefField]bool)
	out := make([]BriefField, 0, len(groups))
	for _, g := range groups {
		for _, f := range g {
			if seen[f] {
				continue
			}
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// ArtifactSpecs returns the catalog, in picker order.
func ArtifactSpecs() []ArtifactSpec {
	out := make([]ArtifactSpec, len(artifactCatalog))
	copy(out, artifactCatalog)
	return out
}

// LookupArtifact returns the spec for an artifact type.
func LookupArtifact(id string) (ArtifactSpec, bool) {
	for _, s := range artifactCatalog {
		if string(s.ID) == id {
			return s, true
		}
	}
	return ArtifactSpec{}, false
}

// IsArtifact reports whether an artifact type is one this deployment runs.
func IsArtifact(id string) bool {
	_, ok := LookupArtifact(id)
	return ok
}

// ArtifactFields returns the fields an artifact accepts, or the full-package
// scope's when the type is unknown.
//
// An unknown artifact resolves to the WIDEST scope rather than the narrowest:
// validating or rendering less than the caller supplied would silently discard
// their input, which is the failure this table exists to prevent. Callers reject
// an unknown artifact separately, with IsArtifact.
func ArtifactFields(id string) []BriefField {
	spec, ok := LookupArtifact(id)
	if !ok {
		spec, _ = LookupArtifact(string(ArtifactFullProblem))
	}
	out := make([]BriefField, len(spec.Fields))
	copy(out, spec.Fields)
	return out
}

// ArtifactIDs returns the artifact type values, sorted, for callers that need to
// enumerate them without the picker's presentation.
func ArtifactIDs() []string {
	out := make([]string, 0, len(artifactCatalog))
	for _, s := range artifactCatalog {
		out = append(out, string(s.ID))
	}
	sort.Strings(out)
	return out
}
