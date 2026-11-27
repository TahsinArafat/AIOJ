package generate

import (
	"fmt"
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
)

// systemPrompt is sent as the "system" role message.
// It defines the model's persona, the exact JSON output schema, and
// rating-band semantics so the model calibrates difficulty correctly.
const systemPrompt = `You are an expert competitive programming problem author specialised in
Codeforces-style problems. You have been trained on thousands of Codeforces problems and editorials.

Your task is to generate a UNIQUE, ORIGINAL competitive programming problem along with its
reference solution and editorial. The problem must be self-contained and solvable within the
given constraints.

## Rating-band guidance
- 800–1000:  trivial implementation or brute force; no non-trivial algorithm required.
- 1100–1300: simple greedy, prefix sums, basic sorting or ad-hoc.
- 1400–1600: standard DP (1D/2D), BFS/DFS, binary search on answer.
- 1700–1900: more complex DP, two pointers, segment trees (basic), combinatorics.
- 2000–2200: advanced data structures, complex graph algorithms, non-trivial math.
- 2300–2500: problem-specific insights, hard constructives, advanced trees or flows.
- 2600–3500: research-level difficulty; unusual observations or heavy algorithmic machinery.

## Output format
Respond with ONLY a valid JSON object matching exactly this schema (no markdown fences, no extra text):

{
  "title": "string — short catchy problem title",
  "description": "string — full Markdown problem statement (scenario and task only; do NOT put the numeric bounds here)",
  "constraints": "string — the numeric bounds as a LaTeX block, one per line, e.g. $1 \\le n \\le 2\\cdot10^5$",
  "input_format": "string — describes the input format (lines/values)",
  "output_format": "string — describes the expected output",
  "hint": "string — optional clarifying hint; empty string if none",
  "sample_cases": [
    {"input": "string", "output": "string", "explanation": "string"}
  ],
  "time_limit_ms": 1000,
  "memory_limit_kb": 262144,
  "tags": ["string"],
  "difficulty": "easy | medium | hard",
  "solution_code": "string — complete, compilable C++17 solution",
  "solution_language": "cpp-gpp-64",
  "editorial_title": "string",
  "editorial_content": "string — step-by-step Markdown editorial explaining the approach",
  "editorial_approach": "string — one-sentence summary of the algorithm used",
  "time_complexity": "string — e.g. O(N log N)",
  "space_complexity": "string — e.g. O(N)"
  ,"generator_code": "string — deterministic C++17 generator; accepts one integer seed argument and writes one testcase to stdout"
  ,"generator_language": "cpp-gpp-64"
  ,"seeds": [1, 2, 3]
  ,"curated_cases": [{"name":"edge-01","input":"string","intent":"what this case verifies","score":10}]
  ,"difficulty_rationale": {
    "core_insight":"string", "prerequisites":["string"],
    "implementation_burden":"string", "likely_wrong_approaches":["string"],
    "constraint_fit":"string"
  }
}

Rules:
- difficulty must be "easy" for rating ≤ 1400, "medium" for ≤ 1900, "hard" for ≥ 2000.
- constraints must list every numeric bound as a LaTeX block (rendered as its own section); do NOT repeat the bounds inside description.
- sample_cases must contain at least 1 and at most 3 examples.
- solution_code must be compilable C++17 and produce the correct answer for the sample cases.
- editorial_content must explain the core insight, NOT just describe the solution code.
- generator_code must be deterministic for a given seed and must not access the network, clock, filesystem, or environment.
- curated case names must be unique safe basenames; do not include extensions or path separators.
- difficulty_rationale must justify why the insight, implementation burden, and constraints fit the requested rating range.
- Do NOT copy existing Codeforces problems verbatim; create a novel scenario.`

// buildUserPrompt constructs the user message from a generation Request.
//
// artifact and existing are not optional decoration: a job on an existing
// problem must be generated against that problem. Before this the legacy worker
// sent only the brief, so "write the reference solution for this problem"
// produced a solution to a problem the model had invented, and Apply wrote it
// onto the real one. The harness worker already passed the problem; this keeps
// both pipelines answering the same question.
func buildUserPrompt(req Request, artifact string, existing *model.Problem) string {
	var b strings.Builder

	if existing != nil {
		fmt.Fprintf(&b, "%s\n\n", artifactInstruction(artifact))
		b.WriteString("## Existing problem — authoritative, reuse its values\n")
		b.WriteString(renderExistingProblem(existing))
		b.WriteString("\n## Operator brief\n")
	} else {
		b.WriteString("Generate a competitive programming problem with these specifications:\n\n")
	}
	fmt.Fprintf(&b, "- Algorithmic topics (tags): %s\n", strings.Join(req.Tags, ", "))
	fmt.Fprintf(&b, "- Difficulty rating: %d (Codeforces scale)\n", req.Rating)
	fmt.Fprintf(&b, "- Problem type: %s\n", req.ProblemType)
	fmt.Fprintf(&b, "- Number of test cases to describe in the statement: %d\n", req.TestCaseCount)
	fmt.Fprintf(&b, "- Enforced profile rules: %s\n", profileInstructions(req))
	if len(req.PrimaryTopics) > 0 {
		fmt.Fprintf(&b, "- Primary topics: %s\n", strings.Join(req.PrimaryTopics, ", "))
	}
	if len(req.SecondaryTopics) > 0 {
		fmt.Fprintf(&b, "- Secondary topics: %s\n", strings.Join(req.SecondaryTopics, ", "))
	}
	if len(req.RequiredTechniques) > 0 {
		fmt.Fprintf(&b, "- Required techniques: %s\n", strings.Join(req.RequiredTechniques, ", "))
	}
	if len(req.ForbiddenTechniques) > 0 {
		fmt.Fprintf(&b, "- Forbidden techniques: %s\n", strings.Join(req.ForbiddenTechniques, ", "))
	}
	if req.RatingMin > 0 || req.RatingMax > 0 {
		fmt.Fprintf(&b, "- Acceptable rating range: %d–%d\n", req.RatingMin, req.RatingMax)
	}
	if req.SolverMode != "" {
		fmt.Fprintf(&b, "- Solver mode: %s (team size %d)\n", req.SolverMode, req.TeamSize)
	}
	if req.EventContext != "" {
		fmt.Fprintf(&b, "- Event context: %s\n", req.EventContext)
	}
	if req.FormatProfile != "" {
		fmt.Fprintf(&b, "- Format profile: %s\n", req.FormatProfile)
	}
	if req.ExpectedSolveMinutes > 0 {
		fmt.Fprintf(&b, "- Expected solve time: %d minutes\n", req.ExpectedSolveMinutes)
	}
	if req.AudienceLevel != "" {
		fmt.Fprintf(&b, "- Audience level: %s\n", req.AudienceLevel)
	}
	if req.InsightEmphasis != "" {
		fmt.Fprintf(&b, "- Insight/implementation emphasis: %s\n", req.InsightEmphasis)
	}
	if req.StoryTone != "" {
		fmt.Fprintf(&b, "- Story tone: %s\n", req.StoryTone)
	}
	if req.ConstraintGuidance != "" {
		fmt.Fprintf(&b, "- Constraint guidance: %s\n", req.ConstraintGuidance)
	}

	if req.HasSubtasks {
		b.WriteString("- Use subtask scoring with 2–4 subtasks (partial credit).\n")
	} else {
		b.WriteString("- Standard all-or-nothing scoring (no subtasks).\n")
	}

	if req.AdditionalPrompt != "" {
		fmt.Fprintf(&b, "- Additional guidance: %s\n", req.AdditionalPrompt)
	}

	b.WriteString("\nRemember: output ONLY the JSON object, nothing else.")
	return b.String()
}

// artifactInstruction states what a single-artifact job must produce, and what
// it must leave alone. A full-package job authors everything, so it gets no
// scope line — the existing problem is context, not a constraint on the output.
func artifactInstruction(artifact string) string {
	switch artifact {
	case "statement":
		return "Rewrite ONLY the statement and its samples for the existing problem below. " +
			"Reuse its constraints, limits and formats verbatim — do not reword them."
	case "tests":
		return "Author ONLY the test generator and cases for the existing problem below. " +
			"Reuse its statement, constraints, limits and samples verbatim — do not reword them."
	case "solution":
		return "Write ONLY the reference solution for the existing problem below. " +
			"It must solve exactly the statement and samples given, within the stated limits."
	case "editorial":
		return "Write ONLY the editorial for the existing problem below. " +
			"It must explain the approach to the statement as given, not to a different problem."
	case "full_problem":
		return "Build the complete package. The existing problem below is context — match its subject, " +
			"and reuse its constraints, limits and formats unless the brief says otherwise."
	default:
		return "Produce the requested artifact for the existing problem below."
	}
}

// renderExistingProblem projects an existing problem into the legacy pipeline's
// user message. Field set mirrors the harness's renderProblem so the two
// pipelines give the model the same view of the problem it is working on.
func renderExistingProblem(p *model.Problem) string {
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
	sb.WriteString(placeholderIfEmpty(p.Description))
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

	if len(p.SampleCases) > 0 {
		sb.WriteString("\n\n### Samples\n")
		for i, s := range p.SampleCases {
			fmt.Fprintf(&sb, "%d. input: %s\n   output: %s\n", i+1, s.Input, s.Output)
			if strings.TrimSpace(s.Explanation) != "" {
				fmt.Fprintf(&sb, "   explanation: %s\n", s.Explanation)
			}
		}
	}
	return sb.String()
}

func placeholderIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(empty)"
	}
	return s
}
