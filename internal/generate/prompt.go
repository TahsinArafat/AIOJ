package generate

import (
	"fmt"
	"strings"
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
  "description": "string — full Markdown problem statement including all constraints",
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
}

Rules:
- difficulty must be "easy" for rating ≤ 1400, "medium" for ≤ 1900, "hard" for ≥ 2000.
- sample_cases must contain at least 1 and at most 3 examples.
- solution_code must be compilable C++17 and produce the correct answer for the sample cases.
- editorial_content must explain the core insight, NOT just describe the solution code.
- Do NOT copy existing Codeforces problems verbatim; create a novel scenario.`

// buildUserPrompt constructs the user message from a generation Request.
func buildUserPrompt(req Request) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Generate a competitive programming problem with these specifications:\n\n")
	fmt.Fprintf(&b, "- Algorithmic topics (tags): %s\n", strings.Join(req.Tags, ", "))
	fmt.Fprintf(&b, "- Difficulty rating: %d (Codeforces scale)\n", req.Rating)
	fmt.Fprintf(&b, "- Problem type: %s\n", req.ProblemType)
	fmt.Fprintf(&b, "- Number of test cases to describe in the statement: %d\n", req.TestCaseCount)

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
