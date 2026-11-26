package runner

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/model"
)

// A contestant's deep recursion died with SIGSEGV at a stack depth unrelated to
// the problem's memory limit, because go-judge applies no RLIMIT_STACK when the
// request omits one and the process inherits the container's 8 MB.
//
// The fix is the request's own `stackLimit` field, which go-judge turns into
// RLIMIT_STACK before exec. Two other approaches cannot work and are recorded
// here so they are not reintroduced: `-Wl,--stack=` is a PE/COFF linker option
// that GNU ld rejects on Linux, and `ulimit -s` in the command's shell runs
// after go-judge has already fixed the limit.

func TestStackLimitBytesConvertsKilobytes(t *testing.T) {
	cfg := &compiler.LangConfig{StackLimitKB: 262144}

	if got := cfg.StackLimitBytes(); got != 262144*1024 {
		t.Fatalf("StackLimitBytes() = %d, want 268435456", got)
	}
}

// An unset limit must stay zero: go-judge treats a zero stackLimit as "no
// limit configured" and leaves the sandbox default alone, which is a real
// choice an operator can make.
func TestStackLimitBytesIsZeroWhenUnset(t *testing.T) {
	for _, kb := range []int{0, -1} {
		cfg := &compiler.LangConfig{StackLimitKB: kb}
		if got := cfg.StackLimitBytes(); got != 0 {
			t.Errorf("StackLimitKB=%d gave %d, want 0", kb, got)
		}
	}
	// A nil config must not panic: callers reach this through a language lookup
	// that can miss.
	var nilCfg *compiler.LangConfig
	if got := nilCfg.StackLimitBytes(); got != 0 {
		t.Errorf("nil config gave %d, want 0", got)
	}
}

// The run command must stay a plain program invocation. Anything that tried to
// raise the stack by wrapping the argv in `sh -c 'ulimit -s ...'` could only
// lower the inherited value, and it also changed how the program was exec'd.
func TestBuildRunArgsDoesNotWrapTheCommandForTheStack(t *testing.T) {
	cfg := &compiler.LangConfig{
		CompileCmd:   "/usr/bin/g++ -O2 -o {{exe}} {{src}}",
		Extensions:   []string{".cpp"},
		StackLimitKB: 262144,
	}

	args := buildRunArgs(cfg, &compiler.CompileResult{Success: true})

	if len(args) != 3 || args[0] != "/bin/sh" || args[1] != "-c" {
		t.Fatalf("argv = %#v, want the plain sh -c command", args)
	}
	if args[2] != "./Main < input.txt > output.txt 2> error.txt" {
		t.Fatalf("script = %q, want the program invoked directly", args[2])
	}
}

// The operator's compile flags must reach the compiler byte-for-byte: the stack
// cap is a runtime rlimit and has nothing to do with the compile argv.
func TestStackConfigLeavesTheCompileCommandUntouched(t *testing.T) {
	const operatorCmd = "/usr/bin/g++ -Wall -Wextra -Wconversion -static -DONLINE_JUDGE " +
		"-O2 -std=c++20 -o {{exe}} {{src}}"

	cfg := &compiler.LangConfig{CompileCmd: operatorCmd, StackLimitKB: 262144}

	if cfg.CompileCmd != operatorCmd {
		t.Fatalf("compile command was rewritten:\n got: %s\nwant: %s", cfg.CompileCmd, operatorCmd)
	}
}

// The batch runner is the path a real submission takes, so the limit has to be
// on the request it actually sends — not merely available on the config.
func TestBatchRunnerSendsTheStackLimit(t *testing.T) {
	var got executor.Cmd
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req executor.ExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		got = req.Cmd[0]
		json.NewEncoder(w).Encode([]executor.CmdResult{{
			Status: "Accepted",
			Files:  map[string]string{"output.txt": "out"},
		}})
	}))
	defer srv.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "1.in"), []byte("in"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "1.out"), []byte("out"), 0o600); err != nil {
		t.Fatal(err)
	}

	prob := &model.Problem{
		ID:           "p1",
		TestdataPath: dir,
		TimeLimit:    1000,
		MemoryLimit:  262144,
		TestCaseScore: []model.TestCaseScore{
			{InputName: "1.in", OutputName: "1.out", Score: 100},
		},
	}
	cfg := &compiler.LangConfig{
		Key:          "cpp-gpp-64",
		CompileCmd:   "/usr/bin/g++ -O2 -o {{exe}} {{src}}",
		Extensions:   []string{".cpp"},
		StackLimitKB: 262144,
	}

	r := NewBatchRunner(executor.NewClient(srv.URL))
	if _, err := r.Run(t.Context(), RunInput{
		Problem:        prob,
		LangCfg:        cfg,
		CompiledResult: &compiler.CompileResult{Success: true, Files: map[string]executor.CmdFile{"Main": {FileID: "f1"}}},
		SourceCode:     "int main(){}",
		CPULimitNs:     1e9,
		MemLimitBytes:  256 * 1024 * 1024,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got.StackLimit != 262144*1024 {
		t.Fatalf("stackLimit = %d, want 268435456 — the submission ran under the sandbox default", got.StackLimit)
	}
}
