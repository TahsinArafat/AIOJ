package problemgen

import (
	"encoding/json"
	"testing"
)

func readyManifest() Manifest {
	return Manifest{
		Limits: Limits{TimeLimitMS: 1500, MemoryLimitKB: 262144},
		Statement: Statement{
			Title:        "Example",
			Description:  "Do the thing.",
			InputFormat:  "n then n integers",
			OutputFormat: "one integer",
			Tags:         []string{"greedy"},
			Difficulty:   "easy",
			Samples:      []Sample{{Input: "3\n1 2 3\n", Output: "6\n"}},
		},
		Cases: []Case{
			{Name: "a.in", Seed: 7},
			{Name: "b.in", Seed: 7},
			{Name: "c.in", Seed: 9},
		},
		Artifacts: map[string]string{
			string(ArtifactGenerator): "gen-source",
			string(ArtifactSolution):  "sol-source",
		},
		Languages: map[string]string{
			string(ArtifactGenerator): "cpp-gpp-64",
			string(ArtifactSolution):  "cpp-gpp-64",
		},
		Report: Report{Ready: true},
	}
}

// A ready harness run must produce a proposal, because the apply path reads the
// proposal column and refuses the job when it is empty.
func TestProposalJSONIsApplyableForReadyManifest(t *testing.T) {
	raw, err := proposalJSON(readyManifest())
	if err != nil {
		t.Fatalf("proposalJSON: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("ready manifest produced an empty proposal")
	}
	var got struct {
		Title             string   `json:"title"`
		Description       string   `json:"description"`
		TimeLimitMs       int      `json:"time_limit_ms"`
		MemoryLimitKB     int      `json:"memory_limit_kb"`
		SolutionCode      string   `json:"solution_code"`
		SolutionLanguage  string   `json:"solution_language"`
		GeneratorCode     string   `json:"generator_code"`
		GeneratorLanguage string   `json:"generator_language"`
		SampleCases       []Sample `json:"sample_cases"`
		Seeds             []int64  `json:"seeds"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Title != "Example" || got.Description != "Do the thing." {
		t.Fatalf("statement not carried over: %+v", got)
	}
	if got.TimeLimitMs != 1500 || got.MemoryLimitKB != 262144 {
		t.Fatalf("limits not carried over: %+v", got)
	}
	if got.SolutionCode != "sol-source" || got.GeneratorCode != "gen-source" {
		t.Fatalf("artifacts not carried over: %+v", got)
	}
	if got.SolutionLanguage != "cpp-gpp-64" || got.GeneratorLanguage != "cpp-gpp-64" {
		t.Fatalf("languages not carried over: %+v", got)
	}
	if len(got.SampleCases) != 1 {
		t.Fatalf("samples = %d, want 1", len(got.SampleCases))
	}
	// Seeds must be de-duplicated so a regeneration replays each one once.
	if len(got.Seeds) != 2 || got.Seeds[0] != 7 || got.Seeds[1] != 9 {
		t.Fatalf("seeds = %v, want [7 9]", got.Seeds)
	}
}

// A run that stopped early must NOT produce a proposal: nothing partially
// validated may be promoted onto a real problem.
func TestProposalJSONIsEmptyForUnreadyManifest(t *testing.T) {
	m := readyManifest()
	m.Report = Report{Ready: false, Blocked: []string{"harness"}}
	raw, err := proposalJSON(m)
	if err != nil {
		t.Fatalf("proposalJSON: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("unready manifest produced a proposal: %s", raw)
	}
}
