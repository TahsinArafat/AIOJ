package problemgen

import (
	"context"
	"fmt"
	"strings"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
)

// Runner is the sandbox seam.
//
// Everything that executes generated code goes through it, which keeps three
// properties true at once: the tools can be tested without a live judge, the
// code only ever runs inside go-judge (never on the API host), and a remote
// deployment can substitute a different sandbox without touching the tools.
type Runner interface {
	// Compile builds one source file and returns the runnable artifacts.
	Compile(ctx context.Context, language, source string) (CompileOutcome, error)
	// Run executes a compiled artifact with the given argv and stdin.
	Run(ctx context.Context, files map[string]executor.CmdFile, args []string, stdin string, lim Limits) (RunOutcome, error)
}

// CompileOutcome is the result of a build.
type CompileOutcome struct {
	OK     bool
	Output string
	Files  map[string]executor.CmdFile
	// Args is the argv that runs the built artifact, derived from the language
	// registry ("./Main" for compiled languages, "python3 Main" for scripts).
	// Callers append their own flags; they never build a shell string.
	Args []string
}

// RunOutcome is the result of one execution.
type RunOutcome struct {
	// Status is the judge's own status string ("Accepted", "TimeLimitExceeded",
	// …) so callers map verdicts the same way the submission pipeline does.
	Status   string
	Stdout   string
	Stderr   string
	TimeMS   int
	MemoryKB int
}

// Accepted reports whether the process exited cleanly within its limits.
func (o RunOutcome) Accepted() bool { return o.Status == "Accepted" }

const maxCapturedOutput = 8 << 20 // 8 MiB, matching the existing AI sandbox cap

// JudgeRunner runs code in go-judge.
type JudgeRunner struct {
	Exec    *executor.Client
	LangDir string
	// langs is loaded once per runner; LoadLanguages reads the YAML registry,
	// which must not happen per call.
	langs map[string]*compiler.LangConfig
}

// NewJudgeRunner loads the language registry.
func NewJudgeRunner(exec *executor.Client, langDir string) (*JudgeRunner, error) {
	langs, err := compiler.LoadLanguages(langDir)
	if err != nil {
		return nil, fmt.Errorf("problemgen: load languages: %w", err)
	}
	return &JudgeRunner{Exec: exec, LangDir: langDir, langs: langs}, nil
}

// Languages exposes the loaded registry for validation of language ids.
func (r *JudgeRunner) Languages() map[string]*compiler.LangConfig { return r.langs }

func (r *JudgeRunner) Compile(ctx context.Context, language, source string) (CompileOutcome, error) {
	cfg := r.langs[language]
	if cfg == nil {
		return CompileOutcome{}, fmt.Errorf("unsupported language %q", language)
	}
	comp := compiler.New(r.Exec)
	res, err := comp.CompileContestant(ctx, source, cfg)
	if err != nil {
		return CompileOutcome{}, fmt.Errorf("compile: %w", err)
	}
	return CompileOutcome{
		OK:     res.Success,
		Output: capText(res.Output, 4096),
		Files:  res.Files,
		Args:   runArgs(cfg),
	}, nil
}

// runArgs derives the argv for a built artifact from the language registry.
// Mirrors internal/judge/runner.buildRunArgs, minus the shell redirection: the
// sandbox takes stdin and captures stdout as files instead.
func runArgs(cfg *compiler.LangConfig) []string {
	if cfg.Runtime == "" {
		return []string{"./Main"}
	}
	parts := strings.Fields(cfg.Runtime)
	for i, p := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(p, "{{dir}}", "."), "{{exe}}", "Main")
	}
	return parts
}

func (r *JudgeRunner) Run(ctx context.Context, files map[string]executor.CmdFile, args []string, stdin string, lim Limits) (RunOutcome, error) {
	copyIn := make(map[string]executor.CmdFile, len(files))
	for k, v := range files {
		copyIn[k] = v
	}

	// Test programs read test data from their process stdin, and both the
	// generator and the reference solution write their answer to process stdout.
	// go-judge expresses those three channels as the positional Cmd.Files
	// triple, so they must be declared here.
	//
	// Two prior bugs came from getting this wrong:
	//   - stdin was handed over as a copyIn *file* named input.txt, which no
	//     program reads, so every solution saw immediate EOF;
	//   - stdout was requested with CopyOut: ["stdout"], but copyOut names real
	//     files in the sandbox, so go-judge failed every run with "File Error".
	stdinFile := executor.CmdFile{Content: stdin}
	stdoutStream := executor.CmdFile{Name: "stdout", Max: maxCapturedOutput}
	stderrStream := executor.CmdFile{Name: "stderr", Max: maxCapturedOutput}

	res, err := r.Exec.Run(ctx, &executor.ExecRequest{Cmd: []executor.Cmd{{
		Args:        args,
		Env:         []string{"PATH=/usr/bin:/bin", "HOME=/tmp"},
		Files:       []executor.CmdFile{stdinFile, stdoutStream, stderrStream},
		CopyIn:      copyIn,
		CPULimit:    uint64(lim.TimeLimitMS) * 1e6,
		MemoryLimit: uint64(lim.MemoryLimitKB) * 1024,
		ProcLimit:   64,
	}}})
	if err != nil {
		return RunOutcome{}, fmt.Errorf("sandbox: %w", err)
	}
	if len(res) == 0 {
		return RunOutcome{}, fmt.Errorf("sandbox returned no result")
	}

	cr := res[0]
	out := RunOutcome{
		Status:   cr.Status,
		Stdout:   cr.Files["stdout"],
		Stderr:   cr.Files["stderr"],
		TimeMS:   int(cr.Time / 1e6),
		MemoryKB: int(cr.Memory / 1024),
	}
	if len(out.Stdout) > maxCapturedOutput {
		return out, fmt.Errorf("output exceeds %d bytes", maxCapturedOutput)
	}
	return out, nil
}

// capText bounds a message so a compiler log cannot flood the model's context.
func capText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// normalizeVerdict maps a judge status to a verdict letter.
func normalizeVerdict(status string) Verdict {
	switch strings.ToLower(status) {
	case "accepted":
		return VerdictAC
	case "time limitexceeded", "time limit exceeded":
		return VerdictTL
	case "nonzero exit status":
		return VerdictRE
	default:
		return VerdictWA
	}
}
