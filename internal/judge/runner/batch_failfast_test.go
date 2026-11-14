package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/model"
)

// fakeSandbox answers each /run with the next canned result and counts the
// calls, so a test can assert on how many cases the judge actually executed.
// Anything past the canned answers is a failure: that is the run that should
// never have happened.
func fakeSandbox(t *testing.T, answers []executor.CmdResult) *executor.Client {
	t.Helper()
	runs := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if runs >= len(answers) {
			t.Errorf("judge executed case %d; the run should have stopped after %d", runs+1, len(answers))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode([]executor.CmdResult{answers[runs]})
		runs++
	}))
	t.Cleanup(srv.Close)
	return executor.NewClient(srv.URL)
}

func writeCases(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	for i := 1; i <= n; i++ {
		name := fmt.Sprintf("%d", i)
		if err := os.WriteFile(filepath.Join(dir, name+".in"), []byte("in"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name+".out"), []byte("out"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func cases(n int, subtask int) []model.TestCaseScore {
	out := make([]model.TestCaseScore, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, model.TestCaseScore{
			InputName:  fmt.Sprintf("%d.in", i),
			OutputName: fmt.Sprintf("%d.out", i),
			Score:      100 / n,
			SubtaskID:  subtask,
		})
	}
	return out
}

func accepted() executor.CmdResult {
	return executor.CmdResult{Status: "Accepted", Files: map[string]string{"output.txt": "out"}}
}

func wrong() executor.CmdResult {
	return executor.CmdResult{Status: "Accepted", Files: map[string]string{"output.txt": "not the answer"}}
}

var testLang = &compiler.LangConfig{
	Key:        "cpp-gpp-64",
	CompileCmd: "g++ -O2 -o {{exe}} {{src}}",
	Extensions: []string{".cpp"},
}

// A complete-scored problem stops at the first rejection: the remaining cases
// are never submitted to the sandbox, and the score is still measured against
// the whole problem rather than only the cases that ran.
func TestBatchRunnerStopsAtFirstRejection(t *testing.T) {
	dir := writeCases(t, 3)
	prob := &model.Problem{
		ID:            "p1",
		TestdataPath:  dir,
		ScoringMode:   "complete",
		TestCaseScore: cases(3, 0),
	}
	// Case 1 passes, case 2 is wrong, case 3 must never run.
	exec := fakeSandbox(t, []executor.CmdResult{accepted(), wrong()})

	out, err := NewBatchRunner(exec).Run(context.Background(), RunInput{
		Problem: prob, LangCfg: testLang, SourceCode: "int main(){}",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(out.Results) != 2 {
		t.Fatalf("ran %d cases, want 2: judging must stop at the first rejection", len(out.Results))
	}
	if got := out.Results[1].Status; got != model.StatusWA {
		t.Errorf("case 2 status = %s, want WA", got)
	}
	if got := out.Results[1].Index; got != 2 {
		t.Errorf("failing case index = %d, want 2", got)
	}
	if out.FinalStatus != model.StatusWA {
		t.Errorf("verdict = %s, want WA", out.FinalStatus)
	}
	// 33 of 100 points: the two unrun cases still count against the total.
	if out.Score != 33 {
		t.Errorf("score = %d, want 33 (earned 33 of the problem's 99 points)", out.Score)
	}
}

// Partial scoring is the one mode that cannot stop early: a subtask's credit is
// computed from every case in it, so the cases after a failure still have to run
// for their own subtasks.
func TestBatchRunnerKeepsRunningPartialScoring(t *testing.T) {
	dir := writeCases(t, 3)
	prob := &model.Problem{
		ID:                 "p1",
		TestdataPath:       dir,
		ScoringMode:        "partial",
		SubtaskAggregation: "min",
		TestCaseScore:      cases(3, 1),
	}
	exec := fakeSandbox(t, []executor.CmdResult{wrong(), accepted(), accepted()})

	out, err := NewBatchRunner(exec).Run(context.Background(), RunInput{
		Problem: prob, LangCfg: testLang, SourceCode: "int main(){}",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(out.Results) != 3 {
		t.Fatalf("ran %d cases, want 3: partial scoring needs every case", len(out.Results))
	}
	if out.FinalStatus != model.StatusWA {
		t.Errorf("verdict = %s, want WA", out.FinalStatus)
	}
}
