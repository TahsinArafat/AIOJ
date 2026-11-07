package hack

import (
	"context"
	"fmt"
	"strings"

	"github.com/tahsinarafat/aioj/internal/judge/checker"
	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/model"
)

// Judge runs one hack: the defender's accepted submission is executed against
// the hacker's test input, and the hack succeeds when the submission crashes,
// times out, exceeds memory, or produces output that differs from the answer
// the hacker supplied.
//
// This replaces the previous stub, which resolved every hack with
// `len(input) % 2 != 0` and returned the literal strings "expected_mock_output"
// and "actual_mock_output". Nothing was executed, so the hack phase was
// decorative.
type Judge struct {
	exec    *executor.Client
	langDir string
}

// NewJudge builds a hack judge. A nil executor or empty langDir leaves the
// judge disabled; Callers must check Enabled() and refuse hacks rather than
// silently accepting them.
func NewJudge(exec *executor.Client, langDir string) *Judge {
	return &Judge{exec: exec, langDir: langDir}
}

// Enabled reports whether the judge can actually run code.
func (j *Judge) Enabled() bool {
	return j != nil && j.exec != nil && j.langDir != ""
}

// Verdict is the outcome of executing a submission against a hack input.
type Verdict struct {
	// Failed is true when the submission did not behave correctly: it crashed,
	// timed out, or produced different output than expected. A hack succeeds
	// exactly when Failed is true.
	Failed bool
	// Status is the submission status observed (ac/re/tle/mle/wa/se).
	Status string
	// ActualOutput is what the submission printed (bounded).
	ActualOutput string
	// Detail explains the verdict for display.
	Detail string
}

// maxCapturedOutput bounds how much of the submission's output is retained.
// A hack input is attacker-controlled, so a submission that prints gigabytes
// must not be copied into the response or the database.
const maxCapturedOutput = 8 << 10

// Run executes sub against testInput for prob and reports whether it failed.
//
// expectedOutput is optional. When the hacker supplies it, the output is
// compared with the problem's configured checker; when it is empty, only a
// crash/timeout/memory failure counts as a successful hack, because without a
// reference answer there is nothing to compare against.
func (j *Judge) Run(ctx context.Context, prob *model.Problem, sub *model.Submission, testInput, expectedOutput string) (Verdict, error) {
	if !j.Enabled() {
		return Verdict{}, fmt.Errorf("hack judging is not configured")
	}

	langs, err := compiler.LoadLanguages(j.langDir)
	if err != nil {
		return Verdict{}, fmt.Errorf("load languages: %w", err)
	}
	cfg := langs[sub.Language]
	if cfg == nil {
		return Verdict{}, fmt.Errorf("unsupported language: %s", sub.Language)
	}

	comp := compiler.New(j.exec)
	compiled, err := comp.CompileContestant(ctx, sub.SourceCode, cfg)
	if err != nil {
		return Verdict{}, fmt.Errorf("compile: %w", err)
	}
	if !compiled.Success {
		// The defender's code was already accepted with this language, so a
		// compile failure now is an infrastructure fault, not a successful hack.
		return Verdict{}, fmt.Errorf("submission no longer compiles: %s", compiled.Output)
	}

	timeLimitMs, memoryLimitKB := effectiveLimits(prob, sub.Language)

	// Build copyIn: the compiled artefacts (or the interpreted source) plus the
	// hack input.
	srcName := "Main" + cfg.Extensions[0]
	copyIn := map[string]executor.CmdFile{srcName: {Content: sub.SourceCode}}
	for name, file := range compiled.Files {
		copyIn[name] = file
	}
	copyIn["input.txt"] = executor.CmdFile{Content: testInput}

	resp, err := j.exec.Run(&executor.ExecRequest{
		Cmd: []executor.Cmd{{
			Args:        buildRunArgs(cfg, compiled),
			Env:         []string{"PATH=/usr/bin:/bin", "HOME=/tmp"},
			CPULimit:    uint64(float64(timeLimitMs)*cfg.TimeLimitMultiplier) * 1_000_000,
			MemoryLimit: uint64(float64(memoryLimitKB)*cfg.MemoryLimitMultiplier) * 1024,
			ProcLimit:   64,
			CopyIn:      copyIn,
			CopyOut:     []string{"output.txt", "error.txt"},
		}},
	})
	if err != nil {
		return Verdict{}, fmt.Errorf("execute: %w", err)
	}
	if len(resp) == 0 {
		return Verdict{}, fmt.Errorf("execute: no result")
	}

	cr := resp[0]
	actual := capOutput(cr.Files["output.txt"])

	switch cr.Status {
	case "TimeLimitExceeded":
		return Verdict{Failed: true, Status: string(model.StatusTLE), ActualOutput: actual,
			Detail: "submission exceeded the time limit on this input"}, nil
	case "MemoryLimitExceeded":
		return Verdict{Failed: true, Status: string(model.StatusMLE), ActualOutput: actual,
			Detail: "submission exceeded the memory limit on this input"}, nil
	case "Nonzero Exit Status", "RuntimeError":
		detail := strings.TrimSpace(cr.Files["error.txt"])
		if detail == "" {
			detail = cr.Error
		}
		if detail == "" {
			detail = "submission exited with a runtime error"
		}
		return Verdict{Failed: true, Status: string(model.StatusRE), ActualOutput: actual,
			Detail: capOutput(detail)}, nil
	case "Accepted":
		// Ran to completion. Without an expected answer there is nothing to
		// compare, so the hack fails: the solution survived the input.
		if strings.TrimSpace(expectedOutput) == "" {
			return Verdict{Failed: false, Status: string(model.StatusAC), ActualOutput: actual,
				Detail: "submission ran successfully; supply expected_output to test for a wrong answer"}, nil
		}
		chk := checker.GetChecker(prob.CheckerType, prob.FloatEpsilon)
		res := chk.Check(nil, []byte(expectedOutput), []byte(actual))
		if res.Passed {
			return Verdict{Failed: false, Status: string(model.StatusAC), ActualOutput: actual,
				Detail: "submission produced the expected output"}, nil
		}
		return Verdict{Failed: true, Status: string(model.StatusWA), ActualOutput: actual,
			Detail: "submission produced different output than expected"}, nil
	default:
		return Verdict{Failed: true, Status: string(model.StatusRE), ActualOutput: actual,
			Detail: cr.Status}, nil
	}
}

// effectiveLimits mirrors the judge's per-language limit resolution.
func effectiveLimits(prob *model.Problem, language string) (timeLimitMs, memoryLimitKB int) {
	timeLimitMs, memoryLimitKB = prob.TimeLimit, prob.MemoryLimit
	for _, ll := range prob.LanguageLimits {
		if ll.LanguageID != language {
			continue
		}
		if ll.TimeLimitMs != nil {
			timeLimitMs = *ll.TimeLimitMs
		}
		if ll.MemoryLimitKB != nil {
			memoryLimitKB = *ll.MemoryLimitKB
		}
		break
	}
	return
}

// buildRunArgs constructs the shell command that runs the submission with the
// hack input on stdin.
func buildRunArgs(cfg *compiler.LangConfig, cr *compiler.CompileResult) []string {
	srcName := "Main" + cfg.Extensions[0]
	tarPrefix := ""
	if cr != nil && cr.TarMode {
		tarPrefix = "tar -xf compile.tar && "
	}
	if cr != nil && cr.Success && cfg.Runtime != "" {
		parts := strings.Fields(cfg.Runtime)
		for i, p := range parts {
			parts[i] = strings.ReplaceAll(p, "{{dir}}", ".")
			parts[i] = strings.ReplaceAll(parts[i], "{{exe}}", "Main")
		}
		return []string{"/bin/sh", "-c", tarPrefix + strings.Join(parts, " ") + " < input.txt > output.txt 2> error.txt"}
	}
	if cfg.Runtime != "" && (cr == nil || !cr.Success) {
		parts := strings.Fields(cfg.Runtime)
		return []string{"/bin/sh", "-c", strings.Join(parts, " ") + " " + srcName + " < input.txt > output.txt 2> error.txt"}
	}
	return []string{"/bin/sh", "-c", tarPrefix + "./Main < input.txt > output.txt 2> error.txt"}
}

// capOutput truncates captured output to a bounded size.
func capOutput(s string) string {
	if len(s) <= maxCapturedOutput {
		return s
	}
	return s[:maxCapturedOutput] + "\n… (truncated)"
}
