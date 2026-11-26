package handler

import (
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
)

// A failed compile has to leave its diagnostics in error.txt.
//
// The compile step used to run bare before `&&`, so a compiler error went to the
// shell's stderr — which the sandbox never returns — and `&&` short-circuited
// before error.txt was created. The IDE then reported "Compilation failed (no
// error output captured)" instead of the error itself.
func TestRunShellCommandCapturesCompilerStderr(t *testing.T) {
	tests := []struct {
		name string
		cfg  *compiler.LangConfig
		want []string
	}{
		{
			name: "compiled, no runtime (c++)",
			cfg: &compiler.LangConfig{
				CompileCmd: "/usr/bin/g++ -O2 -std=c++17 -o {{exe}} {{src}}",
				Extensions: []string{".cpp"},
			},
			want: []string{
				"{ /usr/bin/g++ -O2 -std=c++17 -o Main Main.cpp ; } 2> error.txt && ./Main",
			},
		},
		{
			name: "compiled with runtime (java)",
			cfg: &compiler.LangConfig{
				CompileCmd: "/usr/bin/javac -d {{dir}} {{src}}",
				Runtime:    "/usr/bin/java -cp {{dir}} Main",
				Extensions: []string{".java"},
			},
			want: []string{
				"{ /usr/bin/javac -d . Main.java ; } 2> error.txt && /usr/bin/java -cp . Main",
			},
		},
		{
			name: "interpreted (python)",
			cfg: &compiler.LangConfig{
				Runtime:    "/usr/bin/python3",
				Extensions: []string{".py"},
			},
			want: []string{"/usr/bin/python3 Main.py"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := runShellCommand(tc.cfg, "Main"+tc.cfg.Extensions[0])

			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("command missing %q:\n%s", want, got)
				}
			}
			// Every branch must capture both streams, or a run reports no output
			// and no error.
			if !strings.Contains(got, "> output.txt") {
				t.Errorf("stdout is not captured:\n%s", got)
			}
			if !strings.HasSuffix(got, "2> error.txt") {
				t.Errorf("run stderr is not captured:\n%s", got)
			}
			// A compile step must redirect before `&&`, otherwise a compile
			// failure never reaches error.txt.
			if tc.cfg.CompileCmd != "" {
				compileAt := strings.Index(got, "2> error.txt")
				andAt := strings.Index(got, "&&")
				if compileAt == -1 || andAt == -1 || compileAt > andAt {
					t.Errorf("compile stderr is not redirected before &&:\n%s", got)
				}
			}
		})
	}
}

// The scratchpad must judge under the same stack cap a submission gets, or it
// disagrees with the judge on exactly the deep-recursion cases a contestant is
// using it to debug.
func TestRunShellCommandAppliesTheStackLimit(t *testing.T) {
	cfg := &compiler.LangConfig{
		CompileCmd:   "/usr/bin/g++ -O2 -o {{exe}} {{src}}",
		Extensions:   []string{".cpp"},
		StackLimitKB: 262144,
	}

	got := runShellCommand(cfg, "Main.cpp")

	if !strings.Contains(got, "ulimit -s 262144") {
		t.Fatalf("stack limit missing:\n%s", got)
	}
	// The cap must apply to the program only. A limit set before the compile
	// would also be inherited by g++, which needs a normal stack to run at all.
	if at := strings.Index(got, "ulimit -s"); at > strings.Index(got, "g++") {
		t.Errorf("stack limit is set before the compile runs:\n%s", got)
	}
	// Ungrouped, the `;` would also apply the limit to anything the caller
	// appends after the program.
	if !strings.Contains(got, "{ ulimit -s 262144; ") {
		t.Errorf("stack limit is not scoped to the run:\n%s", got)
	}
}

// A language with no stack_limit_kb keeps the sandbox default; the command must
// be byte-identical to what it was before the knob existed.
func TestRunShellCommandLeavesTheCommandAloneWithoutAStackLimit(t *testing.T) {
	cfg := &compiler.LangConfig{Runtime: "/usr/bin/python3", Extensions: []string{".py"}}

	if got := runShellCommand(cfg, "Main.py"); got != "/usr/bin/python3 Main.py < input.txt > output.txt 2> error.txt" {
		t.Fatalf("command = %q, want it untouched when stack_limit_kb is unset", got)
	}
}

// The placeholders in lang/*.yaml are what make the command runnable; an
// unreplaced one reaches the shell as literal text.
func TestRunShellCommandFillsPlaceholders(t *testing.T) {
	cfg := &compiler.LangConfig{
		CompileCmd: "/usr/bin/mcs -out:{{exe}} {{src}}",
		Runtime:    "/usr/bin/mono {{exe}}",
		Extensions: []string{".cs"},
	}

	got := runShellCommand(cfg, "Main.cs")

	if strings.Contains(got, "{{") {
		t.Fatalf("unreplaced placeholder left in command:\n%s", got)
	}
	if !strings.Contains(got, "/usr/bin/mcs -out:Main Main.cs") {
		t.Errorf("compile placeholders not filled:\n%s", got)
	}
	if !strings.Contains(got, "/usr/bin/mono Main") {
		t.Errorf("runtime placeholders not filled:\n%s", got)
	}
}
