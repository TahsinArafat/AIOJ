// judging engine.  Each problem type (batch, interactive, output-only) has a
// dedicated Runner.
package runner

import (
	"context"
	"strconv"
	"strings"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/model"
)

// RunInput bundles everything a Runner needs to judge a submission.
type RunInput struct {
	Problem        *model.Problem
	LangCfg        *compiler.LangConfig
	CompiledResult *compiler.CompileResult
	SourceCode     string
	SPJFile        executor.CmdFile
	InteractorFile executor.CmdFile
	CPULimitNs     uint64
	MemLimitBytes  uint64
}

// RunOutput is the result of a Runner.Run call.
type RunOutput struct {
	Results     []model.TestCaseResult
	FinalStatus model.SubmissionStatus
	Score       int
	MaxTime     int // ms
	MaxMem      int // KB
}

// Runner judges a submission against all test cases for a single problem.
type Runner interface {
	Run(ctx context.Context, in RunInput) (RunOutput, error)
}

// withStackLimit raises RLIMIT_STACK before the contestant's program runs.
//
// go-judge exposes no rlimit field, so the process inherits the container
// default — 8 MB on Linux. Deep recursion therefore dies with SIGSEGV at a
// depth that has nothing to do with the problem's memory limit, and the
// contestant sees a bare Runtime Error that reads like a judge bug. `ulimit -s`
// run by the same shell that execs the program is the only lever the sandbox
// offers.
//
// Two shapes are accepted because the two runners invoke the program
// differently: the batch runner already goes through `sh -c`, so only the
// script is rewritten; the interactive runner execs an argv directly and needs
// the shell wrapper added.
//
// A config with no stack_limit_kb leaves the sandbox default untouched.
func withStackLimit(args []string, cfg *compiler.LangConfig) []string {
	if cfg.StackLimitKB <= 0 {
		return args
	}
	limit := "ulimit -s " + strconv.Itoa(cfg.StackLimitKB) + " && exec "
	if len(args) >= 3 && args[0] == "/bin/sh" && args[1] == "-c" {
		// Fail loudly rather than running with the old, small stack: a non-zero
		// exit is what the runners already report as a runtime error.
		out := make([]string, len(args))
		copy(out, args)
		out[2] = limit + args[2]
		return out
	}
	return []string{"/bin/sh", "-c", limit + shellQuote(args...)}
}

// shellQuote renders argv as one safely-quoted shell command line. Paths come
// from the language config and the sandbox's own working directory, but quoting
// costs nothing and a space in either would otherwise split an argument.
func shellQuote(args ...string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(quoted, " ")
}
