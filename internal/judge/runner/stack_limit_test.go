package runner

import (
	"strings"
	"testing"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
)

// A contestant's deep recursion used to die with SIGSEGV at a stack depth that
// had nothing to do with the problem's memory limit, because go-judge exposes
// no rlimit and the sandbox inherits the 8 MB container default. stack_limit_kb
// is the config knob that fixes it, and `ulimit -s` in the runner's own shell
// is the only place it can be applied.

func TestStackLimitRewritesTheBatchShellScript(t *testing.T) {
	cfg := &compiler.LangConfig{StackLimitKB: 262144}
	got := withStackLimit([]string{"/bin/sh", "-c", "./Main < input.txt"}, cfg)

	if len(got) != 3 || got[0] != "/bin/sh" || got[1] != "-c" {
		t.Fatalf("argv shape changed: %#v", got)
	}
	if !strings.HasPrefix(got[2], "ulimit -s 262144 && exec ") {
		t.Fatalf("script = %q, want it to raise the stack before exec", got[2])
	}
	// The original command must survive intact — a wrapper that drops it would
	// run nothing and report the user's program as a runtime error.
	if !strings.HasSuffix(got[2], "./Main < input.txt") {
		t.Errorf("script = %q, want the original command preserved", got[2])
	}
}

// The interactive runner execs an argv directly, so the shell wrapper has to be
// introduced rather than rewritten. The contestant's files are addressed by
// absolute sandbox path, which is what makes quoting necessary.
func TestStackLimitWrapsAnArgvItCannotRewrite(t *testing.T) {
	cfg := &compiler.LangConfig{StackLimitKB: 131072}
	got := withStackLimit([]string{"/w/Main"}, cfg)

	if len(got) != 3 || got[0] != "/bin/sh" || got[1] != "-c" {
		t.Fatalf("argv = %#v, want a sh -c wrapper", got)
	}
	if !strings.Contains(got[2], "ulimit -s 131072") {
		t.Errorf("script = %q, want the stack raised", got[2])
	}
	if !strings.Contains(got[2], "'/w/Main'") {
		t.Errorf("script = %q, want the original argv quoted into the exec", got[2])
	}
}

// An unset stack limit must leave the sandbox default completely alone: the
// argv the caller built is what runs.
func TestStackLimitIsIgnoredWhenUnset(t *testing.T) {
	for _, kb := range []int{0, -1} {
		args := []string{"/bin/sh", "-c", "./Main < input.txt"}
		got := withStackLimit(args, &compiler.LangConfig{StackLimitKB: kb})

		if len(got) != 3 || got[2] != "./Main < input.txt" {
			t.Errorf("stack_limit_kb=%d rewrote the command: %#v", kb, got)
		}
	}
}

// A path containing a quote must not break out of the wrapper. The values come
// from the language config, but the quoting is the only thing standing between
// a config typo and an injected command.
func TestStackLimitQuotesHostilePaths(t *testing.T) {
	cfg := &compiler.LangConfig{StackLimitKB: 1024}
	got := withStackLimit([]string{"/w/Main; rm -rf /"}, cfg)

	if strings.Contains(got[2], "; rm -rf / ") && !strings.Contains(got[2], "'/w/Main; rm -rf /'") {
		t.Fatalf("script = %q, want the argument quoted so it cannot run as a command", got[2])
	}
}

// buildRunArgs is the batch path's entry point: every branch must carry the cap,
// or interpreted and compiled languages would be judged under different stacks.
func TestBuildRunArgsAppliesTheStackLimitOnEveryBranch(t *testing.T) {
	cfg := &compiler.LangConfig{
		Key:          "python",
		Extensions:   []string{".py"},
		Runtime:      "/usr/bin/python3",
		StackLimitKB: 262144,
	}
	compiled := &compiler.CompileResult{Success: true}

	for name, args := range map[string][]string{
		"interpreted": buildRunArgs(cfg, nil),
		"compiled":    buildRunArgs(&compiler.LangConfig{Extensions: []string{".cpp"}, StackLimitKB: 262144}, compiled),
	} {
		if len(args) < 3 || !strings.Contains(args[2], "ulimit -s 262144") {
			t.Errorf("%s branch = %#v, want the stack limit applied", name, args)
		}
	}
}

// An operator's compile flags must reach the compiler byte-for-byte. The stack
// cap is a runtime rlimit, so the runner must never rewrite, reorder or append
// to the compile argv to apply it.
func TestStackLimitLeavesTheCompileCommandUntouched(t *testing.T) {
	const operatorCmd = "/usr/bin/g++ -Wall -Wextra -Wconversion -static -DONLINE_JUDGE " +
		"-Wl,--stack=268435456 -O2 -std=c++20 -o {{exe}} {{src}}"

	// Only the placeholders are substituted on the way to the sandbox.
	got := strings.NewReplacer("{{exe}}", "Main", "{{src}}", "Main.cpp").Replace(operatorCmd)

	want := "/usr/bin/g++ -Wall -Wextra -Wconversion -static -DONLINE_JUDGE " +
		"-Wl,--stack=268435456 -O2 -std=c++20 -o Main Main.cpp"
	if got != want {
		t.Fatalf("compile argv was rewritten:\n got: %s\nwant: %s", got, want)
	}
}
