package problemgen

import (
	"encoding/json"
	"fmt"
)

// proposalDoc mirrors generate.Proposal's JSON shape.
//
// The harness stores a Manifest (constraints, cases, artifact sources) while the
// apply path reads a Proposal (statement, solution, generator). Nothing bridged
// the two: SetManifest never wrote the proposal column, so a harness job reached
// status "ready", rendered its gates in AI Studio, and then refused to apply
// with "proposal is invalid" because the column was NULL. This type is that
// bridge.
//
// It deliberately redeclares the fields instead of importing generate: the
// generator packages already depend on this one, and a shared struct would make
// the two pipelines' contracts impossible to evolve independently. Apply unmarshals
// the bytes, so only the JSON tags have to agree.
type proposalDoc struct {
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	Constraints       string            `json:"constraints"`
	InputFormat       string            `json:"input_format"`
	OutputFormat      string            `json:"output_format"`
	Hint              string            `json:"hint"`
	SampleCases       []Sample          `json:"sample_cases"`
	TimeLimitMs       int               `json:"time_limit_ms"`
	MemoryLimitKB     int               `json:"memory_limit_kb"`
	Tags              []string          `json:"tags"`
	Difficulty        string            `json:"difficulty"`
	SolutionCode      string            `json:"solution_code"`
	SolutionLanguage  string            `json:"solution_language"`
	EditorialTitle    string            `json:"editorial_title"`
	EditorialContent  string            `json:"editorial_content"`
	EditorialApproach string            `json:"editorial_approach"`
	GeneratorCode     string            `json:"generator_code"`
	GeneratorLanguage string            `json:"generator_language"`
	Seeds             []int64           `json:"seeds"`
	CuratedCases      []json.RawMessage `json:"curated_cases"`
}

// proposalJSON renders the manifest as an applyable proposal.
//
// Only a ready manifest gets one. A run that stopped on a budget keeps a NULL
// proposal on purpose, so nothing partially-validated can be promoted onto a
// problem by mistake.
func proposalJSON(m Manifest) (json.RawMessage, error) {
	if !m.Report.Ready {
		return nil, nil
	}
	if m.Statement.Title == "" || m.Statement.Description == "" {
		return nil, fmt.Errorf("manifest has no statement to apply")
	}
	doc := proposalDoc{
		Title:             m.Statement.Title,
		Description:       m.Statement.Description,
		Constraints:       m.Statement.Constraints,
		InputFormat:       m.Statement.InputFormat,
		OutputFormat:      m.Statement.OutputFormat,
		Hint:              m.Statement.Hint,
		SampleCases:       m.Statement.Samples,
		TimeLimitMs:       m.Limits.TimeLimitMS,
		MemoryLimitKB:     m.Limits.MemoryLimitKB,
		Tags:              m.Statement.Tags,
		Difficulty:        m.Statement.Difficulty,
		SolutionCode:      m.Artifacts[string(ArtifactSolution)],
		SolutionLanguage:  m.Languages[string(ArtifactSolution)],
		GeneratorCode:     m.Artifacts[string(ArtifactGenerator)],
		GeneratorLanguage: m.Languages[string(ArtifactGenerator)],
		Seeds:             manifestSeeds(m.Cases),
	}
	if doc.SolutionLanguage == "" {
		doc.SolutionLanguage = "cpp-gpp-64"
	}
	if doc.GeneratorLanguage == "" {
		doc.GeneratorLanguage = "cpp-gpp-64"
	}
	// The harness does not author an editorial; leave those fields empty rather
	// than inventing prose the model never wrote.
	return json.Marshal(doc)
}

// manifestSeeds lists the distinct seeds the run materialized, so a future
// regeneration reproduces the same cases.
func manifestSeeds(cases []Case) []int64 {
	seen := make(map[int64]bool, len(cases))
	out := make([]int64, 0, len(cases))
	for _, c := range cases {
		if seen[c.Seed] {
			continue
		}
		seen[c.Seed] = true
		out = append(out, c.Seed)
	}
	return out
}
