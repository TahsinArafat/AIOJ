package problemgen

import (
	"context"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

// A job on an existing problem must carry that problem into the prompt.
//
// Before this, the pipeline received only the operator's brief, so "improve this
// problem" was generated blind and Apply overwrote the original with something
// unrelated.
func TestRenderForCarriesTheExistingProblem(t *testing.T) {
	existing := &model.Problem{
		Title:        "Echo",
		Description:  "Print the first integer of the input.",
		Constraints:  "$1 \\le n \\le 2000$",
		InputFormat:  "n followed by n integers",
		OutputFormat: "the first integer",
		TimeLimit:    1500,
		MemoryLimit:  262144,
		Difficulty:   "easy",
		Tags:         []string{"implementation"},
		SampleCases: []model.SampleCase{
			{Input: "3 1 2 3", Output: "3"},
		},
	}

	out := Brief{Tags: []string{"dp"}, Rating: 1400, AdditionalPrompt: "keep the story about echoes"}.RenderFor("statement", existing)

	for _, want := range []string{
		"Echo",
		"Print the first integer of the input.",
		"$1 \\le n \\le 2000$",
		"1500 ms",
		"262144",
		"implementation",
		"3 1 2 3",
		// The operator's brief must survive alongside the context — as long as it
		// is a field this artifact uses. Guidance is accepted by every scope.
		"keep the story about echoes",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("prompt missing %q:\n%s", want, out)
		}
	}
}

// The per-artifact input split. Every field used to be rendered for every
// artifact, so a statement rewrite was told "Topics: dp, Target rating: 1400"
// beside "Existing problem … Tags: implementation" — an instruction to rebuild
// the problem around inputs the operator was never asked for, contradicting the
// problem it was supposed to be rewriting.
func TestRenderForRendersOnlyFieldsTheArtifactUses(t *testing.T) {
	// Everything set, so any rendered line is the artifact's own doing.
	full := Brief{
		Tags: []string{"dp"}, Rating: 1400, RatingMin: 1300, RatingMax: 1500,
		FormatProfile: "codeforces", SolverMode: "solo", EventContext: "practice",
		TestCaseCount: 20, HasSubtasks: true, AudienceLevel: "advanced",
		StoryTone: "playful", ConstraintGuidance: "N up to 2e5",
		ForbiddenTechniques: []string{"flows"}, AdditionalPrompt: "extra steer",
	}

	cases := map[string]struct {
		want    []string
		notWant []string
	}{
		// The artifact that invents a problem needs the inputs that define one.
		"full_problem": {
			want: []string{"Topics: dp", "Target rating: 1400", "test-set size", "Subtask scoring", "Audience: advanced"},
		},
		// A statement rewrite works from the existing problem, so the defining
		// inputs are neither asked for nor rendered; register is.
		"statement": {
			want:    []string{"Audience: advanced", "Story tone: playful", "Must not require: flows"},
			notWant: []string{"Target rating", "Format profile", "test-set size", "Subtask scoring"},
		},
		"tests": {
			want:    []string{"test-set size", "Subtask scoring", "Must not require: flows"},
			notWant: []string{"Target rating", "Audience:", "Story tone"},
		},
		"solution": {
			want:    []string{"Must not require: flows", "Constraint guidance"},
			notWant: []string{"Target rating", "test-set size", "Audience:", "Story tone"},
		},
		// An editorial explains a problem that already exists: no topic, no
		// rating, no test count — this is the case the report named.
		"editorial": {
			want:    []string{"Audience: advanced", "Story tone: playful"},
			notWant: []string{"Topics: dp", "Target rating", "test-set size", "Format profile", "Must not require"},
		},
	}

	for artifact, tc := range cases {
		out := full.RenderFor(artifact, nil)
		for _, want := range tc.want {
			if !strings.Contains(out, want) {
				t.Fatalf("%s: prompt missing %q:\n%s", artifact, want, out)
			}
		}
		for _, notWant := range tc.notWant {
			if strings.Contains(out, notWant) {
				t.Fatalf("%s: prompt must not contain %q:\n%s", artifact, notWant, out)
			}
		}
	}
}

// AdditionalPrompt is the one field every artifact takes: it is the escape hatch
// for steering a run, so it must never be gated away.
func TestAdditionalPromptSurvivesEveryArtifact(t *testing.T) {
	for _, a := range model.ArtifactIDs() {
		out := Brief{Tags: []string{"dp"}, AdditionalPrompt: "unique steer"}.RenderFor(a, nil)
		if !strings.Contains(out, "unique steer") {
			t.Fatalf("artifact %q dropped the additional prompt:\n%s", a, out)
		}
	}
}

func TestRenderForStatesTheArtifactScope(t *testing.T) {
	cases := map[string]string{
		"statement": "Rewrite the statement",
		"tests":     "test suite",
		"solution":  "reference solution",
		"editorial": "editorial",
	}
	for artifact, want := range cases {
		out := Brief{Tags: []string{"dp"}}.RenderFor(artifact, nil)
		if !strings.Contains(out, want) {
			t.Fatalf("artifact %q: prompt does not describe its scope, missing %q:\n%s", artifact, want, out)
		}
	}

	// An unknown artifact must still produce a usable prompt rather than an
	// empty scope line.
	out := Brief{Tags: []string{"dp"}}.RenderFor("", nil)
	if !strings.Contains(out, "complete package") {
		t.Fatalf("empty artifact should fall back to the full package:\n%s", out)
	}
}

// testcase_count was collected by the brief form and validated, then never
// rendered — the harness silently ignored the requested test-set size.
func TestRenderCarriesTheRequestedTestCaseCount(t *testing.T) {
	out := Brief{Tags: []string{"dp"}, TestCaseCount: 20}.RenderFor("full_problem", nil)
	if !strings.Contains(out, "Target test-set size") || !strings.Contains(out, "20 cases") {
		t.Fatalf("brief does not carry testcase_count:\n%s", out)
	}
	if !strings.Contains(out, "seeds_per_cell") {
		t.Fatalf("brief must say how to reach the count:\n%s", out)
	}

	// Absent means "let the matrix decide", so no line is emitted.
	if out := (Brief{Tags: []string{"dp"}}).RenderFor("full_problem", nil); strings.Contains(out, "Target test-set size") {
		t.Fatalf("an unset testcase_count must not be rendered:\n%s", out)
	}
}

func TestRenderForStatesTheDeploymentNMaxCap(t *testing.T) {
	// The cap is configuration, so it belongs in the per-job message rather than
	// the constant system prompt the model sees every turn.
	out := Brief{Tags: []string{"dp"}}.RenderFor("full_problem", nil)
	if !strings.Contains(out, "must not exceed") {
		t.Fatalf("prompt does not state the n_max cap:\n%s", out)
	}
}

// corpusStub answers Similar from a fixed list.
type corpusStub struct {
	hits []SimilarProblem
	err  error
}

func (c *corpusStub) Similar(context.Context, string, string, int) ([]SimilarProblem, error) {
	return c.hits, c.err
}

// search_problems used to be a tool the model could simply skip, so a near
// duplicate could be published. It is a gate at finalize now.
func TestFinalizeBlocksANearDuplicate(t *testing.T) {
	ws := newTestWorkspace(t, &fakeRunner{})
	tools := NewTools(ws, &corpusStub{hits: []SimilarProblem{
		{Slug: "echo", Title: "Echo", Similarity: 0.97},
	}})
	buildProblem(t, tools)

	_, err := run(t, tools, "finalize", `{}`)
	if err == nil {
		t.Fatal("finalize published a problem that duplicates one in the corpus")
	}
	if !strings.Contains(err.Error(), "novelty") {
		t.Fatalf("the failure should name the gate: %v", err)
	}
}

func TestFinalizeAllowsANovelProblem(t *testing.T) {
	ws := newTestWorkspace(t, &fakeRunner{})
	tools := NewTools(ws, &corpusStub{hits: []SimilarProblem{
		{Slug: "unrelated", Title: "Unrelated", Similarity: 0.10},
	}})
	buildProblem(t, tools)

	if _, err := run(t, tools, "finalize", `{}`); err != nil {
		t.Fatalf("a novel problem was blocked: %v", err)
	}
}

func TestNoveltyGateSkipsWithoutACorpus(t *testing.T) {
	// A deployment with no corpus index must not be blocked by the gate.
	tools := newTestTools(t)
	buildProblem(t, tools)
	checks := tools.noveltyCheck(context.Background())
	if len(checks) != 0 {
		t.Fatalf("checks = %+v, want none without a corpus", checks)
	}
}
