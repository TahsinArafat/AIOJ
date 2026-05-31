package model

import "time"

type SampleCase struct {
	Input       string `json:"input"`
	Output      string `json:"output"`
	Explanation string `json:"explanation,omitempty"`
}

type TestCaseScore struct {
	InputName    string `json:"input_name"`
	OutputName   string `json:"output_name"`
	Score        int    `json:"score"`
	SubtaskID    int    `json:"subtask_id,omitempty"`    // 0 = no subtask
	SubtaskScore int    `json:"subtask_score,omitempty"` // score within subtask
}

type Problem struct {
	ID              string          `json:"id"`
	Slug            string          `json:"slug"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	InputFormat     string          `json:"input_format,omitempty"`
	OutputFormat    string          `json:"output_format,omitempty"`
	Hint            string          `json:"hint,omitempty"`
	SampleCases     []SampleCase    `json:"sample_cases"`
	TimeLimit       int             `json:"time_limit"`
	MemoryLimit     int             `json:"memory_limit"`
	Difficulty      string          `json:"difficulty"`
	Tags            []string        `json:"tags,omitempty"`
	Visible         bool            `json:"visible"`
	TestdataPath    string          `json:"-"`
	TestCaseScore   []TestCaseScore `json:"testcase_score,omitempty"`
	SPJ             bool            `json:"spj"`
	SPJLanguage     string          `json:"spj_language,omitempty"`
	SPJSourceCode   string          `json:"spj_source_code,omitempty"`
	SPJVersion      string          `json:"spj_version,omitempty"`
	CheckerType     string          `json:"checker_type"`
	FloatEpsilon    float64         `json:"float_epsilon"`
	Interactive          bool   `json:"interactive"`
	InteractorLanguage   string `json:"interactor_language,omitempty"`
	InteractorSourceCode string `json:"interactor_source_code,omitempty"`
	ScoringMode        string `json:"scoring_mode"`         // "complete" or "partial"
	SubtaskAggregation string `json:"subtask_aggregation"`  // "min" or "sum"
	LanguageLimits     []LanguageLimit `json:"language_limits,omitempty"`
	SubmissionCount int             `json:"submission_count"`
	AcceptedCount   int             `json:"accepted_count"`
	Source          string          `json:"source"`
	RemoteID        string          `json:"remote_id,omitempty"`
	CreatedBy       string          `json:"created_by"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type ProblemListItem struct {
	ID              string   `json:"id"`
	Slug            string   `json:"slug"`
	Title           string   `json:"title"`
	Difficulty      string   `json:"difficulty"`
	Tags            []string `json:"tags"`
	SubmissionCount int      `json:"submission_count"`
	AcceptedCount   int      `json:"accepted_count"`
	Source          string   `json:"source"`
}

type ProblemWithSamples struct {
	Problem
	Index string `json:"index"`
}

type ProblemPermission struct {
	ProblemID   string `json:"problem_id"`
	UserID      string `json:"user_id"`
	Username    string `json:"username,omitempty"`
	AccessLevel string `json:"access_level"`
}

type CreateProblemRequest struct {
	Slug          string          `json:"slug"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	InputFormat   string          `json:"input_format,omitempty"`
	OutputFormat  string          `json:"output_format,omitempty"`
	Hint          string          `json:"hint,omitempty"`
	TimeLimit     int             `json:"time_limit"`
	MemoryLimit   int             `json:"memory_limit"`
	Difficulty    string          `json:"difficulty"`
	Tags          []string        `json:"tags,omitempty"`
	SampleCases   []SampleCase    `json:"sample_cases,omitempty"`
	TestCaseScore []TestCaseScore `json:"testcase_score,omitempty"`
	SPJ           bool            `json:"spj"`
	SPJLanguage   string          `json:"spj_language,omitempty"`
	SPJSourceCode string          `json:"spj_source_code,omitempty"`
	CheckerType   string          `json:"checker_type,omitempty"`
	FloatEpsilon  float64         `json:"float_epsilon,omitempty"`
	Interactive          bool   `json:"interactive"`
	InteractorLanguage   string `json:"interactor_language,omitempty"`
	InteractorSourceCode string `json:"interactor_source_code,omitempty"`
	ScoringMode        string `json:"scoring_mode,omitempty"`
	SubtaskAggregation string `json:"subtask_aggregation,omitempty"`
}

type ProblemStats struct {
	TotalSubmissions    int            `json:"total_submissions"`
	AcceptedSubmissions int            `json:"accepted_submissions"`
	AcceptanceRate      float64        `json:"acceptance_rate"`
	UniqueSolvers       int            `json:"unique_solvers"`
	AverageAttempts     float64        `json:"average_attempts"`
	LanguageDistribution map[string]int `json:"language_distribution"`
	DifficultyEstimate  float64        `json:"difficulty_estimate"`
}

// GetSubtasks returns test cases grouped by subtask_id.
// Cases with SubtaskID == 0 are not grouped.
func (p *Problem) GetSubtasks() map[int][]TestCaseScore {
	subtasks := make(map[int][]TestCaseScore)
	for _, tc := range p.TestCaseScore {
		if tc.SubtaskID > 0 {
			subtasks[tc.SubtaskID] = append(subtasks[tc.SubtaskID], tc)
		}
	}
	return subtasks
}

// HasSubtasks returns true if any test case has SubtaskID > 0.
func (p *Problem) HasSubtasks() bool {
	for _, tc := range p.TestCaseScore {
		if tc.SubtaskID > 0 {
			return true
		}
	}
	return false
}

type UserProblemStats struct {
	ProblemsSolved   int     `json:"problems_solved"`
	TotalSubmissions int     `json:"total_submissions"`
	AcceptanceRate   float64 `json:"acceptance_rate"`
	FavoriteLanguage string  `json:"favorite_language"`
	StreakDays       int     `json:"streak_days"`
}

type PlatformStats struct {
	Problems    int `json:"problems"`
	Users       int `json:"users"`
	Submissions int `json:"submissions"`
}
