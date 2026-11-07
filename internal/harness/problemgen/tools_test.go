package problemgen

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/harness"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
)

func call(name, args string) ai.ToolCall {
	return ai.ToolCall{ID: "c-" + name, Type: "function", Function: ai.ToolCallFunction{Name: name, Arguments: args}}
}

func run(t *testing.T, tools *Tools, name, args string) (harness.Result, error) {
	t.Helper()
	return tools.Execute(context.Background(), call(name, args))
}

// buildProblem drives the catalog the way a model would, up to (but not
// including) finalize.
func buildProblem(t *testing.T, tools *Tools) {
	t.Helper()
	steps := []struct{ name, args string }{
		{"set_constraints", `{"n_min":1,"n_max":1000,"value_min":-1,"value_max":1000}`},
		{"set_limits", `{"time_limit_ms":2000,"memory_limit_kb":262144}`},
		{"set_generator_shapes", `{"shapes":["random","reverse"]}`},
		{"write_generator", `{"language":"cpp-gpp-64","source":"GENERATOR"}`},
		{"write_solution", `{"language":"cpp-gpp-64","source":"SOLUTION"}`},
		{"set_statement", `{"title":"Echo","description":"Print the first integer.","input_format":"n","output_format":"n","tags":["implementation"],"difficulty":"easy","samples":[{"input":"3 1 2 3","output":"3"}]}`},
		{"materialize_matrix", `{}`},
	}
	for _, s := range steps {
		if _, err := run(t, tools, s.name, s.args); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
	}
}

func newTestTools(t *testing.T) *Tools {
	t.Helper()
	ws := newTestWorkspace(t, &fakeRunner{})
	return NewTools(ws, nil)
}

func TestCatalogExposesNoShellTool(t *testing.T) {
	// The security invariant: the model picks what to do next, never what gets
	// executed. Every tool is named and validated; there is no escape hatch.
	tools := newTestTools(t)
	for _, tool := range tools.Tools() {
		switch tool.Function.Name {
		case "run_shell", "exec", "bash", "run_code", "read_file", "write_file":
			t.Fatalf("catalog exposes a dangerous tool: %s", tool.Function.Name)
		}
		// Every tool must declare a closed schema.
		params := tool.Function.Parameters
		if params["type"] != "object" {
			t.Fatalf("%s: parameters must be an object", tool.Function.Name)
		}
		if params["additionalProperties"] != false {
			t.Fatalf("%s: schema must set additionalProperties:false", tool.Function.Name)
		}
		if _, ok := params["required"]; !ok {
			t.Fatalf("%s: schema must declare required fields", tool.Function.Name)
		}
	}
}

func TestFinalizePassesForACompleteProblem(t *testing.T) {
	tools := newTestTools(t)
	buildProblem(t, tools)

	res, err := run(t, tools, "finalize", `{}`)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}

	var body struct {
		OK     bool   `json:"ok"`
		Report Report `json:"report"`
	}
	if err := json.Unmarshal([]byte(res.Content), &body); err != nil {
		t.Fatalf("invalid result JSON: %v (%s)", err, res.Content)
	}
	if !body.OK || !body.Report.Ready {
		t.Fatalf("report not ready: %+v", body.Report)
	}
	if len(body.Report.Cases) < 5 {
		t.Fatalf("only %d cases materialized", len(body.Report.Cases))
	}

	// Every gate must have run and passed.
	want := map[string]bool{"coverage": false, "determinism": false, "limits": false, "samples": false}
	for _, c := range body.Report.Checks {
		if _, ok := want[c.Name]; ok {
			if !c.Passed {
				t.Fatalf("gate %s failed: %s", c.Name, c.Message)
			}
			want[c.Name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("gate %s did not run", name)
		}
	}
}

func TestFinalizeBlocksWithoutAMaxCase(t *testing.T) {
	tools := newTestTools(t)
	if _, err := run(t, tools, "set_constraints", `{"n_min":1,"n_max":1000,"value_min":-1,"value_max":1000}`); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, tools, "set_limits", `{"time_limit_ms":2000,"memory_limit_kb":262144}`); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, tools, "write_generator", `{"language":"cpp-gpp-64","source":"GENERATOR"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, tools, "write_solution", `{"language":"cpp-gpp-64","source":"SOLUTION"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, tools, "set_statement", `{"title":"Echo","description":"d","input_format":"i","output_format":"o","tags":["t"],"difficulty":"easy","samples":[{"input":"3","output":"3"}]}`); err != nil {
		t.Fatal(err)
	}
	// Only a small case: the constraint never gets exercised.
	if _, err := run(t, tools, "generate_case", `{"seed":1,"n":10,"shape":"random","tier":"small"}`); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, tools, "finalize", `{}`)
	if err == nil {
		t.Fatal("expected finalize to block")
	}
	if !strings.Contains(err.Error(), "coverage") {
		t.Fatalf("the failure should name the gate: %v", err)
	}
}

func TestFinalizeBlocksOnAWrongSample(t *testing.T) {
	tools := newTestTools(t)
	buildProblem(t, tools)
	// Rewrite the statement with a sample the reference cannot reproduce.
	if _, err := run(t, tools, "set_statement", `{"title":"Echo","description":"d","input_format":"i","output_format":"o","tags":["t"],"difficulty":"easy","samples":[{"input":"3 1 2 3","output":"999"}]}`); err != nil {
		t.Fatal(err)
	}
	_, err := run(t, tools, "finalize", `{}`)
	if err == nil || !strings.Contains(err.Error(), "samples") {
		t.Fatalf("err = %v, want a sample failure", err)
	}
}

func TestWriteArtifactReportsCompileErrorToTheModel(t *testing.T) {
	tools := newTestTools(t)
	_, err := run(t, tools, "write_generator", `{"language":"cpp-gpp-64","source":"COMPILE_ERROR"}`)
	if err == nil {
		t.Fatal("expected an error")
	}
	// The compile log is the actionable part; the model fixes the code from it.
	if !strings.Contains(err.Error(), "expected ';'") {
		t.Fatalf("error should carry the compiler log: %v", err)
	}
}

func TestUnknownToolIsAnError(t *testing.T) {
	tools := newTestTools(t)
	if _, err := run(t, tools, "rm_rf", `{}`); err == nil {
		t.Fatal("an unknown tool must fail")
	}
}

func TestMalformedArgumentsAreRecoverable(t *testing.T) {
	tools := newTestTools(t)
	_, err := run(t, tools, "set_limits", `not json`)
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("err = %v", err)
	}
}

func TestGenerateCaseReturnsMetadataOnly(t *testing.T) {
	// The bytes must never enter the context window: a 1e5 case is ~1 MB.
	tools := newTestTools(t)
	if _, err := run(t, tools, "set_constraints", `{"n_min":1,"n_max":1000,"value_min":-1,"value_max":1000}`); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, tools, "write_generator", `{"language":"cpp-gpp-64","source":"GENERATOR"}`); err != nil {
		t.Fatal(err)
	}

	res, err := run(t, tools, "generate_case", `{"seed":1,"n":1000,"shape":"random","tier":"medium"}`)
	if err != nil {
		t.Fatalf("generate_case: %v", err)
	}
	// A bounded preview is the point; the body is not.
	if len(res.Content) > 1200 {
		t.Fatalf("result is %d bytes; case bodies must not be returned", len(res.Content))
	}
	if res.BytesMaterialized < 2000 {
		t.Fatalf("materialized %d bytes, expected a ~2 KB case", res.BytesMaterialized)
	}
	if len(res.Content) > int(res.BytesMaterialized)/2 {
		t.Fatalf("result (%d bytes) is not meaningfully smaller than the case (%d bytes)", len(res.Content), res.BytesMaterialized)
	}
	if res.BytesMaterialized <= 0 {
		t.Fatal("the result must report how many bytes were written")
	}
}

func TestSetConstraintsRejectsAnUnboundedN(t *testing.T) {
	tools := newTestTools(t)
	// The judge loads a whole input into memory, so an unbounded n is a
	// host-memory risk rather than only a disk one.
	_, err := run(t, tools, "set_constraints", `{"n_min":1,"n_max":99999999,"value_min":0,"value_max":10}`)
	if err == nil {
		t.Fatal("expected a rejection")
	}
}

func TestSetLimitsRejectsAZeroTimeLimit(t *testing.T) {
	tools := newTestTools(t)
	// A zero time limit is not "unlimited" here: the judge uses it as the CPU
	// limit, so a proposal omitting it would produce an unjudgeable problem.
	if _, err := run(t, tools, "set_limits", `{"time_limit_ms":0,"memory_limit_kb":262144}`); err == nil {
		t.Fatal("expected a rejection")
	}
}

func TestSearchProblemsWithoutACorpusSaysSo(t *testing.T) {
	tools := newTestTools(t)
	res, err := run(t, tools, "search_problems", `{"title":"Echo"}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, `"available":false`) {
		t.Fatalf("result = %s", res.Content)
	}
}

// stubCorpus reports a fixed similarity.
type stubCorpus struct{ sim float64 }

func (s stubCorpus) Similar(_ context.Context, title, _ string, _ int) ([]SimilarProblem, error) {
	return []SimilarProblem{{Slug: "two-sum", Title: title + " (existing)", Similarity: s.sim}}, nil
}

func TestSearchProblemsFlagsALikelyDuplicate(t *testing.T) {
	ws := newTestWorkspace(t, &fakeRunner{})
	tools := NewTools(ws, stubCorpus{sim: 0.95})

	res, err := run(t, tools, "search_problems", `{"title":"Two Sum"}`)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		LikelyDuplicate []SimilarProblem `json:"likely_duplicate"`
	}
	if err := json.Unmarshal([]byte(res.Content), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.LikelyDuplicate) != 1 {
		t.Fatalf("a 0.95 match must be flagged: %s", res.Content)
	}
}

func TestSearchProblemsPassesANovelProblem(t *testing.T) {
	ws := newTestWorkspace(t, &fakeRunner{})
	tools := NewTools(ws, stubCorpus{sim: 0.2})

	res, _ := run(t, tools, "search_problems", `{"title":"Novel"}`)
	var body struct {
		LikelyDuplicate []SimilarProblem `json:"likely_duplicate"`
	}
	if err := json.Unmarshal([]byte(res.Content), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.LikelyDuplicate) != 0 {
		t.Fatalf("a 0.2 match must not be flagged: %s", res.Content)
	}
}

func TestCrosscheckCatchesAWrongReferenceSolution(t *testing.T) {
	// The failure that actually makes a problem bad, and the one no amount of
	// determinism or limit checking can find.
	ws := newTestWorkspace(t, &wrongSolutionRunner{})
	tools := NewTools(ws, nil)

	if _, err := run(t, tools, "set_constraints", `{"n_min":1,"n_max":100,"value_min":0,"value_max":1000}`); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, tools, "set_limits", `{"time_limit_ms":2000,"memory_limit_kb":262144}`); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, tools, "write_generator", `{"language":"cpp-gpp-64","source":"GENERATOR"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, tools, "write_solution", `{"language":"cpp-gpp-64","source":"SOLUTION"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, tools, "write_brute", `{"language":"cpp-gpp-64","source":"BRUTE"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, tools, "set_statement", `{"title":"Echo","description":"d","input_format":"i","output_format":"o","tags":["t"],"difficulty":"easy","samples":[{"input":"3","output":"3"}]}`); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, tools, "materialize_matrix", `{}`); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, tools, "finalize", `{}`)
	if err == nil || !strings.Contains(err.Error(), "crosscheck") {
		t.Fatalf("err = %v, want a crosscheck failure", err)
	}
}

// wrongSolutionRunner makes the brute force disagree with the reference while
// both remain individually consistent — the case only a cross-check can catch.
type wrongSolutionRunner struct{ fakeRunner }

func (w *wrongSolutionRunner) Run(ctx context.Context, files map[string]executor.CmdFile, args []string, stdin string, lim Limits) (RunOutcome, error) {
	if strings.Contains(files["Main"].Content, "BRUTE") {
		return RunOutcome{Status: "Accepted", Stdout: "0\n"}, nil
	}
	return w.fakeRunner.Run(ctx, files, args, stdin, lim)
}
