package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
)

// "Test Samples" used to compile and run in one sandbox process, under the run
// limit — 256 MB and 5 s for C++. A real -O2 compile exceeds that, so cc1plus
// was OOM-killed and the panel reported "g++: fatal error: Killed signal
// terminated program cc1plus" as a compile error on code the judge compiles
// happily. Compiling and running are now separate sandbox calls, each with the
// budget the judge gives that phase.

// TestCompileForRunUsesTheJudgeCompileBudget pins the memory and CPU a compile
// gets. The numbers are the compiler package's own, so a compile cannot be
// starved relative to the judge path.
func TestCompileForRunUsesTheJudgeCompileBudget(t *testing.T) {
	var got executor.Cmd
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req executor.ExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		got = req.Cmd[0]
		json.NewEncoder(w).Encode([]executor.CmdResult{{
			Status:  "Accepted",
			FileIDs: map[string]string{"Main": "file-1"},
		}})
	}))
	defer srv.Close()

	h := &SubmissionHandler{exec: executor.NewClient(srv.URL)}
	cfg := &compiler.LangConfig{
		Key:        "cpp-gpp-64",
		CompileCmd: "/usr/bin/g++ -O2 -o {{exe}} {{src}}",
		Extensions: []string{".cpp"},
	}

	if _, failed := h.compileForRun(t.Context(), "int main(){}", cfg); failed != nil {
		t.Fatalf("compile failed: %+v", failed)
	}

	if got.MemoryLimit != 512*1024*1024 {
		t.Errorf("compile memory limit = %d, want 512 MB — below this g++ is OOM-killed", got.MemoryLimit)
	}
	if got.CPULimit < 30*1e9 {
		t.Errorf("compile CPU limit = %d ns, want at least 30 s", got.CPULimit)
	}
}

// A language with no compile step must not spend a sandbox call compiling.
func TestCompileForRunSkipsInterpretedLanguages(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode([]executor.CmdResult{{Status: "Accepted"}})
	}))
	defer srv.Close()

	h := &SubmissionHandler{exec: executor.NewClient(srv.URL)}
	cfg := &compiler.LangConfig{Key: "python", Runtime: "/usr/bin/python3", Extensions: []string{".py"}}

	compiled, failed := h.compileForRun(t.Context(), "print(1)", cfg)
	if failed != nil {
		t.Fatalf("unexpected failure: %+v", failed)
	}
	if compiled != nil {
		t.Errorf("compiled = %+v, want nil for a language with no compile step", compiled)
	}
	if calls != 0 {
		t.Errorf("%d sandbox calls for an interpreted language, want 0", calls)
	}
}

// A compile that fails must come back as CE with the compiler's own diagnostics,
// not as a runtime error from the run phase.
func TestCompileForRunReportsCEWithCompilerOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]executor.CmdResult{{
			Status:     "Nonzero Exit Status",
			ExitStatus: 1,
			Files:      map[string]string{"error.txt": "Main.cpp:1:1: error: expected ';'"},
		}})
	}))
	defer srv.Close()

	h := &SubmissionHandler{exec: executor.NewClient(srv.URL)}
	cfg := &compiler.LangConfig{CompileCmd: "/usr/bin/g++ -o {{exe}} {{src}}", Extensions: []string{".cpp"}}

	_, failed := h.compileForRun(t.Context(), "int main(){", cfg)
	if failed == nil {
		t.Fatal("a failed compile returned no error response")
	}
	if failed.Status != "CE" {
		t.Errorf("status = %q, want CE", failed.Status)
	}
	if !strings.Contains(failed.CompileOutput, "expected ';'") {
		t.Errorf("compile output = %q, want the compiler's diagnostics", failed.CompileOutput)
	}
}

// The run call must carry the stack limit as a sandbox rlimit. go-judge applies
// rlimits before exec, so this is the only value that has any effect.
func TestRunPreparedSendsTheStackLimit(t *testing.T) {
	var got executor.Cmd
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req executor.ExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		got = req.Cmd[0]
		json.NewEncoder(w).Encode([]executor.CmdResult{{Status: "Accepted"}})
	}))
	defer srv.Close()

	h := &SubmissionHandler{exec: executor.NewClient(srv.URL)}
	cfg := &compiler.LangConfig{
		CompileCmd:   "/usr/bin/g++ -O2 -o {{exe}} {{src}}",
		Extensions:   []string{".cpp"},
		StackLimitKB: 262144,
	}

	h.runPrepared(t.Context(), CustomRunRequest{Input: "1"}, cfg,
		&compiler.CompileResult{Success: true, Files: map[string]executor.CmdFile{"Main": {FileID: "f1"}}})

	if got.StackLimit != 262144*1024 {
		t.Errorf("stackLimit = %d, want 262144 KB in bytes", got.StackLimit)
	}
	// The compiled artefact has to travel with the run, or the sandbox has
	// nothing to execute.
	if _, ok := got.CopyIn["Main"]; !ok {
		t.Errorf("copyIn = %v, want the compiled artefact", got.CopyIn)
	}
}

// An unset stack limit must leave the field at zero so the sandbox default
// stands; sending 0 explicitly is what "no limit configured" means to go-judge.
func TestRunPreparedOmitsAnUnsetStackLimit(t *testing.T) {
	var got executor.Cmd
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req executor.ExecRequest
		json.NewDecoder(r.Body).Decode(&req)
		got = req.Cmd[0]
		json.NewEncoder(w).Encode([]executor.CmdResult{{Status: "Accepted"}})
	}))
	defer srv.Close()

	h := &SubmissionHandler{exec: executor.NewClient(srv.URL)}
	cfg := &compiler.LangConfig{Runtime: "/usr/bin/python3", Extensions: []string{".py"}}

	h.runPrepared(t.Context(), CustomRunRequest{Input: "1"}, cfg, nil)

	if got.StackLimit != 0 {
		t.Errorf("stackLimit = %d, want 0 when the language sets none", got.StackLimit)
	}
}

// buildRunInvocation is the batch path's command builder: every branch has to
// capture both streams and read input.txt, or a run reports nothing.
func TestBuildRunInvocationCapturesBothStreams(t *testing.T) {
	cases := map[string]*compiler.LangConfig{
		"compiled":           {CompileCmd: "/usr/bin/g++ -o {{exe}} {{src}}", Extensions: []string{".cpp"}},
		"compiled + runtime": {CompileCmd: "/usr/bin/javac -d {{dir}} {{src}}", Runtime: "/usr/bin/java -cp {{dir}} Main", Extensions: []string{".java"}},
		"interpreted":        {Runtime: "/usr/bin/python3", Extensions: []string{".py"}},
	}

	for name, cfg := range cases {
		inv := buildRunInvocation(cfg, "Main"+cfg.Extensions[0], "code", &compiler.CompileResult{Success: true})
		script := inv.Args[len(inv.Args)-1]

		if !strings.Contains(script, "< input.txt") {
			t.Errorf("%s: command does not read input.txt:\n%s", name, script)
		}
		if !strings.Contains(script, "> output.txt") {
			t.Errorf("%s: stdout is not captured:\n%s", name, script)
		}
		if !strings.Contains(script, "2> error.txt") {
			t.Errorf("%s: stderr is not captured:\n%s", name, script)
		}
		if strings.Contains(script, "{{") {
			t.Errorf("%s: unreplaced placeholder:\n%s", name, script)
		}
	}
}

// The tarball Java's compile produces has to be unpacked before the run, or
// `java -cp . Main` finds no classes.
func TestBuildRunInvocationUnpacksTarMode(t *testing.T) {
	cfg := &compiler.LangConfig{
		CompileCmd: "/usr/bin/javac -d {{dir}} {{src}}",
		Runtime:    "/usr/bin/java -cp {{dir}} Main",
		Extensions: []string{".java"},
	}

	inv := buildRunInvocation(cfg, "Main.java", "code",
		&compiler.CompileResult{Success: true, TarMode: true, Files: map[string]executor.CmdFile{"compile.tar": {FileID: "f1"}}})

	if !strings.Contains(inv.Args[len(inv.Args)-1], "tar -xf compile.tar") {
		t.Errorf("command does not unpack the class archive:\n%s", inv.Args[len(inv.Args)-1])
	}
	if _, ok := inv.CopyIn["compile.tar"]; !ok {
		t.Errorf("copyIn = %v, want the archive", inv.CopyIn)
	}
}
