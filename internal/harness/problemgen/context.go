package problemgen

import (
	"fmt"
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
)

// artifactScopes states what each artifact_type asks the model to produce.
//
// The harness previously ignored artifact_type entirely, so choosing "Statement
// & samples" in AI Studio still built a whole problem and Apply then wrote only
// the statement. The scope keeps the work proportional to the request.
//
// Every scope still has to satisfy the gates (coverage needs a min and a max
// tier, finalize needs materialized cases), so each one says how to stay minimal
// rather than implying the work can be skipped.
var artifactScopes = map[string]string{
	"full_problem": "Build the complete package: statement, samples, generator, test data and reference solution.",
	"statement":    "Rewrite the statement and its samples. Reuse the existing constraints and limits verbatim. The gates still need cases, so materialize a minimal matrix — one shape is enough.",
	"tests":        "Author the test suite: generator, shapes and case matrix. Reuse the existing statement and constraints verbatim — do not reword them.",
	"solution":     "Write the reference solution for the existing statement and limits. The tests are not the deliverable, so keep the matrix minimal — one shape.",
	"editorial":    "Write the editorial: approach, reasoning and complexity. Call write_solution so the code is compiled and agreed with the samples, and keep the matrix minimal.",
}

// RenderFor composes the user message for one job: the artifact scope, the
// existing problem when the job targets one, and the operator's brief.
//
// Deployment limits are appended here rather than in SystemPrompt so the system
// prompt stays a constant byte-for-byte prefix (it is re-sent every turn and is
// the cacheable part), while the limits follow the running configuration.
func (b Brief) RenderFor(artifact string, existing *model.Problem) string {
	var sb strings.Builder

	scope, ok := artifactScopes[artifact]
	if !ok {
		scope = artifactScopes["full_problem"]
	}
	sb.WriteString("## What to produce\n")
	sb.WriteString(scope)
	sb.WriteString("\n\n")

	// Every artifact still has to pass the same gates, so say so before the
	// model plans a shortcut that finalize will reject.
	sb.WriteString("Regardless of scope, finalize still requires declared constraints, limits, " +
		"shapes and at least one materialized case.")
	if existing != nil {
		sb.WriteString(" Reuse the existing problem's values instead of inventing new ones.")
	}
	sb.WriteString("\n")
	if MaxN > 0 {
		fmt.Fprintf(&sb, "n_max must not exceed %d on this deployment.\n", MaxN)
	}

	if existing != nil {
		sb.WriteString("\n## Existing problem — authoritative, reuse its values\n")
		sb.WriteString(renderProblem(existing))
	}

	sb.WriteString("\n\n## Operator brief\n")
	sb.WriteString(b.renderIntent())
	sb.WriteString("\nNothing is published until a human reviews the result.\n")
	return sb.String()
}

// renderProblem projects an existing problem into the model's input.
//
// This is the difference between "improve this problem" and "write an unrelated
// one": before it, a job on an existing problem was generated blind and Apply
// overwrote the statement with whatever came back.
func renderProblem(p *model.Problem) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Title: %s\n", p.Title)
	if p.Difficulty != "" {
		fmt.Fprintf(&sb, "Difficulty: %s\n", p.Difficulty)
	}
	if len(p.Tags) > 0 {
		fmt.Fprintf(&sb, "Tags: %s\n", strings.Join(p.Tags, ", "))
	}
	if p.TimeLimit > 0 || p.MemoryLimit > 0 {
		fmt.Fprintf(&sb, "Limits: %d ms, %d KiB\n", p.TimeLimit, p.MemoryLimit)
	}

	sb.WriteString("\n### Statement\n")
	sb.WriteString(orPlaceholder(p.Description))
	if strings.TrimSpace(p.Constraints) != "" {
		sb.WriteString("\n\n### Constraints\n")
		sb.WriteString(p.Constraints)
	}
	if strings.TrimSpace(p.InputFormat) != "" {
		sb.WriteString("\n\n### Input format\n")
		sb.WriteString(p.InputFormat)
	}
	if strings.TrimSpace(p.OutputFormat) != "" {
		sb.WriteString("\n\n### Output format\n")
		sb.WriteString(p.OutputFormat)
	}
	if strings.TrimSpace(p.Hint) != "" {
		sb.WriteString("\n\n### Current notes field\n")
		sb.WriteString(p.Hint)
	}

	if len(p.SampleCases) > 0 {
		sb.WriteString("\n\n### Current samples\n")
		for i, s := range p.SampleCases {
			fmt.Fprintf(&sb, "%d. input: %s\n   output: %s\n", i+1, s.Input, s.Output)
			if strings.TrimSpace(s.Explanation) != "" {
				fmt.Fprintf(&sb, "   explanation: %s\n", s.Explanation)
			}
		}
	}
	return sb.String()
}

func orPlaceholder(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(empty)"
	}
	return s
}
