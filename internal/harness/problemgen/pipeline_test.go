package problemgen

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/harness"
)

// toolScriptModel replays a scripted sequence of tool calls, then answers.
//
// Each entry is one assistant turn; the model answers once the script runs out.
// It lets a test drive the real catalog the way a model would, without a live
// endpoint.
type toolScriptModel struct {
	script [][]ai.ToolCall
	turn   int
	// prompts records the user prompt seen on the first call.
	prompts []string
}

func (m *toolScriptModel) CompleteWithTools(_ context.Context, messages []ai.Message, _ []ai.Tool) (*ai.CompletionResult, error) {
	if m.turn == 0 {
		for _, msg := range messages {
			if msg.Role == ai.RoleUser {
				m.prompts = append(m.prompts, msg.Content)
				break
			}
		}
	}
	if m.turn >= len(m.script) {
		return &ai.CompletionResult{Content: "finished", FinishReason: ai.FinishStop}, nil
	}
	calls := m.script[m.turn]
	m.turn++
	return &ai.CompletionResult{ToolCalls: calls, FinishReason: ai.FinishToolCalls}, nil
}

func tc(id, name, args string) ai.ToolCall {
	return ai.ToolCall{ID: id, Type: "function", Function: ai.ToolCallFunction{Name: name, Arguments: args}}
}

// happyScript is the sequence a competent model would produce.
func happyScript() [][]ai.ToolCall {
	return [][]ai.ToolCall{
		{tc("1", "set_constraints", `{"n_min":1,"n_max":2000,"value_min":-1,"value_max":1000}`)},
		{tc("2", "set_limits", `{"time_limit_ms":2000,"memory_limit_kb":262144}`)},
		{tc("3", "set_generator_shapes", `{"shapes":["random","reverse"]}`)},
		{tc("4", "write_generator", `{"language":"cpp-gpp-64","source":"GENERATOR"}`)},
		{tc("5", "write_solution", `{"language":"cpp-gpp-64","source":"SOLUTION"}`)},
		{tc("6", "set_statement", "{\"title\":\"Echo\",\"description\":\"Print the first integer.\",\"constraints\":\"$1 \\\\\\\\le n \\\\\\\\le 2000$\\n\\n$-1 \\\\\\\\le a_i \\\\\\\\le 1000$\",\"input_format\":\"n\",\"output_format\":\"n\",\"tags\":[\"implementation\"],\"difficulty\":\"easy\",\"samples\":[{\"input\":\"3 1 2 3\",\"output\":\"3\"}]}")},
		{tc("7", "materialize_matrix", `{}`)},
		{tc("8", "finalize", `{}`)},
	}
}

func newTestPipeline(t *testing.T, model harness.Model) *Pipeline {
	t.Helper()
	return &Pipeline{
		Model:       model,
		Runner:      &fakeRunner{},
		Transcript:  harness.NewMemoryTranscript(),
		StagingRoot: t.TempDir(),
		MaxBytes:    8 << 20,
	}
}

func TestPipelineProducesAReadyManifest(t *testing.T) {
	p := newTestPipeline(t, &toolScriptModel{script: happyScript()})

	m, err := p.Run(context.Background(), "job-42", "Make an easy echo problem.")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !m.Report.Ready {
		t.Fatalf("manifest not ready: %+v", m.Report.Checks)
	}
	if m.Statement.Title != "Echo" {
		t.Fatalf("title = %q", m.Statement.Title)
	}
	if m.Constraints.NMax != 2000 {
		t.Fatalf("n_max = %d", m.Constraints.NMax)
	}
	if len(m.Cases) < 8 {
		t.Fatalf("only %d cases", len(m.Cases))
	}

	// The manifest must be durable enough to regenerate the problem.
	for _, kind := range []Artifact{ArtifactGenerator, ArtifactSolution} {
		if m.Artifacts[string(kind)] == "" {
			t.Fatalf("artifact %s missing from the manifest", kind)
		}
		if m.Languages[string(kind)] != "cpp-gpp-64" {
			t.Fatalf("language for %s = %q", kind, m.Languages[string(kind)])
		}
	}
	// Every case carries a hash, so apply can verify what it promotes.
	for _, c := range m.Cases {
		if c.SHA256 == "" {
			t.Fatalf("case %s has no hash", c.Name)
		}
	}
}

func TestPipelineMarksAPartialRunAsNotReady(t *testing.T) {
	// The model stopped calling tools before finalize; the pipeline must not
	// hand an unvalidated problem to review.
	script := happyScript()[:6] // up to set_statement, no matrix, no finalize
	p := newTestPipeline(t, &toolScriptModel{script: script})

	m, err := p.Run(context.Background(), "job-43", "brief")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if m.Report.Ready {
		t.Fatal("a run without cases must not be ready")
	}
	var sawCoverage bool
	for _, c := range m.Report.Checks {
		if c.Name == "coverage" && !c.Passed {
			sawCoverage = true
		}
	}
	if !sawCoverage {
		t.Fatalf("coverage should be the reported blocker: %+v", m.Report.Checks)
	}
}

func TestPipelineReportsABudgetStop(t *testing.T) {
	// A model that never stops asking for tools is the default failure mode of
	// an unbounded loop; the run must end with a reason, not hang.
	var script [][]ai.ToolCall
	for i := 0; i < 50; i++ {
		script = append(script, []ai.ToolCall{tc(fmt.Sprint(i), "validate_cases", `{}`)})
	}
	p := newTestPipeline(t, &toolScriptModel{script: script})
	p.Budget = harness.Budget{MaxTurns: 4}

	m, err := p.Run(context.Background(), "job-44", "brief")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if m.Report.Ready {
		t.Fatal("a budget stop must not be ready")
	}
	if len(m.Report.Checks) == 0 || !strings.Contains(m.Report.Checks[0].Message, "stopped early") {
		t.Fatalf("checks = %+v", m.Report.Checks)
	}
}

func TestPipelineSystemPromptStatesTheHardConstraints(t *testing.T) {
	model := &toolScriptModel{script: nil}
	p := newTestPipeline(t, model)
	if _, err := p.Run(context.Background(), "job-45", "the brief"); err != nil {
		t.Fatalf("run: %v", err)
	}

	prompt := SystemPrompt
	// The two things a model cannot infer and must be told.
	if !strings.Contains(prompt, "cannot be written by you") {
		t.Fatal("the prompt must explain that large cases come from the generator")
	}
	if !strings.Contains(prompt, "--seed") || !strings.Contains(prompt, "--n") {
		t.Fatal("the prompt must state the generator contract")
	}
	if !strings.Contains(prompt, "deterministic") {
		t.Fatal("the prompt must require determinism")
	}
	// The problem page renders statements with remark-math + KaTeX, which only
	// typesets $...$. Without this instruction the model wrote math in backticks
	// and readers saw the literal characters "10^9".
	if !strings.Contains(prompt, "$") {
		t.Fatal("the prompt must ask for $...$ LaTeX math")
	}
	if !strings.Contains(prompt, "backtick") {
		t.Fatal("the prompt must forbid inline-code spans for mathematics")
	}
	// memory_limit_kb is KiB; the page divides by 1024 to show MB.
	if !strings.Contains(prompt, "262144") {
		t.Fatal("the prompt must state the KiB value for 256 MB")
	}
	if len(model.prompts) != 1 || model.prompts[0] != "the brief" {
		t.Fatalf("brief not forwarded: %v", model.prompts)
	}
}

func TestPipelineRequiresARunnerAndRoot(t *testing.T) {
	if _, err := (&Pipeline{Model: &toolScriptModel{}, StagingRoot: t.TempDir()}).Run(context.Background(), "j", "b"); err == nil {
		t.Fatal("expected an error without a runner")
	}
	if _, err := (&Pipeline{Model: &toolScriptModel{}, Runner: &fakeRunner{}}).Run(context.Background(), "j", "b"); err == nil {
		t.Fatal("expected an error without a staging root")
	}
}

func TestManifestJSONIsStable(t *testing.T) {
	// The manifest is persisted on the job and read back by apply; it must
	// round-trip without losing the fields apply depends on.
	p := newTestPipeline(t, &toolScriptModel{script: happyScript()})
	m, err := p.Run(context.Background(), "job-46", "brief")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var back Manifest
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Cases) != len(m.Cases) {
		t.Fatalf("cases lost in round-trip: %d vs %d", len(back.Cases), len(m.Cases))
	}
	if back.Limits.TimeLimitMS != m.Limits.TimeLimitMS {
		t.Fatal("limits lost in round-trip")
	}
	if back.Statement.Title != m.Statement.Title {
		t.Fatal("statement lost in round-trip")
	}
	// Paths must not be persisted: they are staging-relative and meaningless
	// once the job's directory is promoted or removed.
	if strings.Contains(string(raw), m.Cases[0].Path) && m.Cases[0].Path != "" {
		t.Fatal("case paths leaked into the manifest")
	}
}
