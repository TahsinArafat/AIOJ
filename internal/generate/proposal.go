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

// ProposalJSONSchema returns a JSON Schema describing Proposal.
//
// It previously returned {"type":"object"}, which is the same as "anything
// goes": a strict-mode provider had nothing to constrain the response with, so
// the model could omit required fields and the failure only surfaced later in
// ParseAndValidateProposal as a generic validation error. The schema below
// declares every field with its type and marks the ones the prompt relies on as
// required, so a spec-compliant endpoint produces a usable artifact on the
// first turn.
//
// additionalProperties is false to match Proposal's DisallowUnknownFields
// decoding: a schema that permitted extra keys would let the model emit fields
// the decoder then rejects.
func ProposalJSONSchema() map[string]any {
	stringField := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"title", "description", "input_format", "output_format"},
		"properties": map[string]any{
			"title":              stringField("Problem title"),
			"description":        stringField("Full problem statement"),
			"input_format":       stringField("Description of the input format"),
			"output_format":      stringField("Description of the output format"),
			"hint":               stringField("Optional short hint"),
			"time_limit_ms":      map[string]any{"type": "integer", "minimum": 100, "description": "CPU time limit in milliseconds"},
			"memory_limit_kb":    map[string]any{"type": "integer", "minimum": 1024, "description": "Memory limit in kilobytes"},
			"tags":               map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"difficulty":         stringField("Difficulty band, e.g. easy/medium/hard"),
			"solution_code":      stringField("Reference solution source"),
			"solution_language":  stringField("Language key of the reference solution"),
			"editorial_title":    stringField("Editorial heading"),
			"editorial_content":  stringField("Editorial body"),
			"editorial_approach": stringField("High-level approach"),
			"time_complexity":    stringField("e.g. O(n log n)"),
			"space_complexity":   stringField("e.g. O(n)"),
			"generator_code":     stringField("Test-generator source"),
			"generator_language": stringField("Language key of the generator"),
			"seeds": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "integer"},
				"description": "Deterministic seeds for the generator",
			},
			"sample_cases": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"input", "output"},
					"properties": map[string]any{
						"input":       map[string]any{"type": "string"},
						"output":      map[string]any{"type": "string"},
						"explanation": map[string]any{"type": "string"},
					},
				},
			},
			"curated_cases": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"name", "input"},
					"properties": map[string]any{
						"name":   map[string]any{"type": "string"},
						"input":  map[string]any{"type": "string"},
						"intent": map[string]any{"type": "string"},
						"score":  map[string]any{"type": "integer"},
					},
				},
			},
			"difficulty_rationale": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"core_insight":          map[string]any{"type": "string"},
					"prerequisites":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"implementation_burden": map[string]any{"type": "string"},
					"likely_wrong_approaches": map[string]any{
						"type":  "array",
						"items": map[string]any{"type": "string"},
					},
					"constraint_fit": map[string]any{"type": "string"},
				},
			},
		},
	}
}
