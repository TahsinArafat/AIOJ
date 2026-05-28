package model

// LanguageLimit defines per-language resource limit overrides for a problem.
// If TimeLimitMs is nil, the problem's default time limit applies.
// If MemoryLimitKB is nil, the problem's default memory limit applies.
type LanguageLimit struct {
	ProblemID     string `json:"problem_id"`
	LanguageID    string `json:"language_id"`
	TimeLimitMs   *int   `json:"time_limit_ms,omitempty"`
	MemoryLimitKB *int   `json:"memory_limit_kb,omitempty"`
}
