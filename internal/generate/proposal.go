package generate

import (
	"encoding/json"
	"fmt"
	"github.com/tahsinarafat/aioj/internal/model"
	"path/filepath"
	"strings"
)

type CuratedCase struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Intent string `json:"intent"`
	Score  int    `json:"score"`
}
type DifficultyRationale struct {
	CoreInsight           string   `json:"core_insight"`
	Prerequisites         []string `json:"prerequisites"`
	ImplementationBurden  string   `json:"implementation_burden"`
	LikelyWrongApproaches []string `json:"likely_wrong_approaches"`
	ConstraintFit         string   `json:"constraint_fit"`
}
type ValidationCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}
type ValidationReport struct {
	Valid  bool              `json:"valid"`
	Checks []ValidationCheck `json:"checks"`
}

type Proposal struct {
	Title               string              `json:"title"`
	Description         string              `json:"description"`
	InputFormat         string              `json:"input_format"`
	OutputFormat        string              `json:"output_format"`
	Hint                string              `json:"hint"`
	SampleCases         []model.SampleCase  `json:"sample_cases"`
	TimeLimitMs         int                 `json:"time_limit_ms"`
	MemoryLimitKB       int                 `json:"memory_limit_kb"`
	Tags                []string            `json:"tags"`
	Difficulty          string              `json:"difficulty"`
	SolutionCode        string              `json:"solution_code"`
	SolutionLanguage    string              `json:"solution_language"`
	EditorialTitle      string              `json:"editorial_title"`
	EditorialContent    string              `json:"editorial_content"`
	EditorialApproach   string              `json:"editorial_approach"`
	TimeComplexity      string              `json:"time_complexity"`
	SpaceComplexity     string              `json:"space_complexity"`
	GeneratorCode       string              `json:"generator_code"`
	GeneratorLanguage   string              `json:"generator_language"`
	Seeds               []int64             `json:"seeds"`
	CuratedCases        []CuratedCase       `json:"curated_cases"`
	DifficultyRationale DifficultyRationale `json:"difficulty_rationale"`
}

func ParseAndValidateProposal(raw, artifact string) (*Proposal, ValidationReport, error) {
	var p Proposal
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, ValidationReport{}, fmt.Errorf("invalid proposal JSON: %w", err)
	}
	report := ValidateProposal(&p, artifact)
	if !report.Valid {
		return &p, report, fmt.Errorf("proposal failed validation")
	}
	return &p, report, nil
}

func ValidateProposal(p *Proposal, artifact string) ValidationReport {
	checks := []ValidationCheck{}
	add := func(n string, ok bool, m string) {
		checks = append(checks, ValidationCheck{Name: n, Passed: ok, Message: m})
	}
	ws := artifact == "full_problem" || artifact == "statement"
	wsol := artifact == "full_problem" || artifact == "solution" || artifact == "tests"
	we := artifact == "full_problem" || artifact == "editorial"
	wt := artifact == "full_problem" || artifact == "tests"
	if ws {
		add("statement", p.Title != "" && p.Description != "" && p.InputFormat != "" && p.OutputFormat != "", "title, statement, input and output formats are required")
		add("samples", len(p.SampleCases) >= 1 && len(p.SampleCases) <= 3, "one to three samples required")
	}
	if wsol {
		add("reference_solution", p.SolutionCode != "" && p.SolutionLanguage != "", "reference solution and language required")
	}
	if we {
		add("editorial", p.EditorialTitle != "" && p.EditorialContent != "", "editorial title and content required")
	}
	if wt {
		add("generator", p.GeneratorCode != "" && p.GeneratorLanguage != "" && len(p.Seeds) > 0, "deterministic generator and seeds required")
		add("curated_cases", len(p.CuratedCases) > 0, "at least one curated edge case required")
		safe := true
		seen := map[string]bool{}
		for _, c := range p.CuratedCases {
			if c.Name == "" || filepath.Base(c.Name) != c.Name || strings.Contains(c.Name, "..") || seen[c.Name] || c.Input == "" {
				safe = false
			}
			seen[c.Name] = true
		}
		add("safe_case_names", safe, "case names must be unique basenames")
	}
	valid := true
	for _, c := range checks {
		if !c.Passed {
			valid = false
		}
	}
	return ValidationReport{Valid: valid, Checks: checks}
}

func ProposalJSONSchema() map[string]any { return map[string]any{"type": "object"} }
