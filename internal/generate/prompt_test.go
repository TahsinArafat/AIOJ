package generate

import (
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

// A job on an existing problem must be generated against that problem.
//
// The legacy worker sent only the brief, so "write the reference solution for
// this problem" produced a solution to a problem the model had invented and
// Apply wrote it onto the real one. The statement and samples are the whole
// point: without them the model cannot solve the right problem.
func TestBuildUserPromptCarriesTheExistingProblem(t *testing.T) {
	existing := &model.Problem{
		Title:        "The Xeno Glyph Signatures",
		Description:  "Given a string of glyphs, count the distinct signatures.",
		Constraints:  `$1 \le n \le 2\cdot10^5$`,
		InputFormat:  "The first line contains n.",
		OutputFormat: "Print the count.",
		TimeLimit:    2000,
		MemoryLimit:  262144,
		SampleCases: []model.SampleCase{
			{Input: "3\nabc", Output: "3"},
			{Input: "1\nz", Output: "1", Explanation: "one glyph, one signature"},
		},
	}

	out := buildUserPrompt(Request{Tags: []string{"dp"}}, "solution", existing)

	for _, want := range []string{
		"The Xeno Glyph Signatures",
		"count the distinct signatures",
		`$1 \le n \le 2\cdot10^5$`,
		"The first line contains n.",
		"Print the count.",
		"abc",
		"one glyph, one signature",
		"reference solution",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prompt is missing %q\n---\n%s", want, out)
		}
	}
}

// A full-package job on a new problem has nothing to be based on, and must not
// claim an existing problem it was not given.
func TestBuildUserPromptWithoutAnExistingProblem(t *testing.T) {
	out := buildUserPrompt(Request{Tags: []string{"dp"}}, "full_problem", nil)
	if strings.Contains(out, "Existing problem") {
		t.Errorf("a job with no problem must not render an existing-problem section:\n%s", out)
	}
	if !strings.Contains(out, "Generate a competitive programming problem") {
		t.Errorf("a new-problem job lost its authoring instruction:\n%s", out)
	}
}

// Each single-artifact scope has to say what to write and what to leave alone,
// or the model rewrites the whole problem for a one-section request.
func TestArtifactInstructionScopesTheWork(t *testing.T) {
	cases := map[string]string{
		"statement": "statement",
		"tests":     "test generator",
		"solution":  "reference solution",
		"editorial": "editorial",
	}
	for artifact, want := range cases {
		if got := artifactInstruction(artifact); !strings.Contains(got, want) {
			t.Errorf("artifactInstruction(%q) = %q, want it to mention %q", artifact, got, want)
		}
	}
}
