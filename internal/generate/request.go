package generate

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
	AdditionalPrompt string `json:"additional_prompt,omitempty"`
}

// GenerateResult is returned after a successful generation + DB save.
type GenerateResult struct {
	ProblemID    string `json:"problem_id"`
	ProblemSlug  string `json:"problem_slug"`
	EditorialID  string `json:"editorial_id"`
}
