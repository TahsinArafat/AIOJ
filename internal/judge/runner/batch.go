package runner

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tahsinarafat/aioj/internal/judge/checker"
	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/model"
)

// BatchRunner judges standard (non-interactive, non-output-only) problems.
// It supports both flat test-case scoring and subtask-based partial scoring.
type BatchRunner struct {
	exec   *executor.Client
	scorer *Scorer
}

// NewBatchRunner returns a BatchRunner backed by the given executor client.
func NewBatchRunner(exec *executor.Client) *BatchRunner {
	return &BatchRunner{exec: exec, scorer: NewScorer()}
}

// Run implements Runner.
func (b *BatchRunner) Run(ctx context.Context, in RunInput) (RunOutput, error) {
	prob := in.Problem
	cfg := in.LangCfg

	// Build the initial copyIn map (source file for interpreted languages).
	srcName := "Main" + cfg.Extensions[0]
	copyIn := map[string]executor.CmdFile{srcName: {Content: in.SourceCode}}

	// Collect test cases — either flat or subtask-ordered.
	var orderedCases []model.TestCaseScore
	if prob.HasSubtasks() && prob.ScoringMode == "partial" {
		orderedCases = subtaskOrderedCases(prob)
	} else {
		orderedCases = prob.TestCaseScore
	}

	results := make([]model.TestCaseResult, 0, len(orderedCases))
	for _, tc := range orderedCases {
		r := b.runOne(ctx, prob, cfg, tc, copyIn, in)
		results = append(results, r)
	}

	out := b.scorer.Aggregate(prob, results)
	return out, nil
}

// runOne executes a single test case and returns its result.
func (b *BatchRunner) runOne(
	ctx context.Context,
	prob *model.Problem,
	langCfg *compiler.LangConfig,
	tc model.TestCaseScore,
	copyIn map[string]executor.CmdFile,
	in RunInput,
) model.TestCaseResult {
	// Inject compiled artefacts into copyIn.
	if in.CompiledResult != nil && in.CompiledResult.Success {
		for fname, file := range in.CompiledResult.Files {
			copyIn[fname] = file
		}
	}

	args := buildRunArgs(langCfg, in.CompiledResult)

	inputContent := loadFile(filepath.Join(prob.TestdataPath, tc.InputName))
	copyIn["input.txt"] = executor.CmdFile{Content: inputContent}

	start := time.Now()
	resp, err := b.exec.Run(&executor.ExecRequest{
		Cmd: []executor.Cmd{{
			Args:        args,
			Env:         []string{"PATH=/usr/bin:/bin", "HOME=/tmp"},
			CPULimit:    in.CPULimitNs,
			MemoryLimit: in.MemLimitBytes,
			ProcLimit:   64,
			CopyIn:      copyIn,
			CopyOut:     []string{"output.txt", "error.txt"},
		}},
	})
	elapsed := int(time.Since(start).Milliseconds())

	r := model.TestCaseResult{CaseName: tc.InputName, Time: elapsed}

	if err != nil {
		r.Status = model.StatusSE
		r.Detail = err.Error()
		return r
	}
	if len(resp) == 0 {
		r.Status = model.StatusSE
		r.Detail = "no result"
		return r
	}

	cr := resp[0]
	r.Memory = int(cr.Memory / 1024)

	switch cr.Status {
	case "Accepted":
		output := cr.Files["output.txt"]
		expected := loadFile(filepath.Join(prob.TestdataPath, tc.OutputName))
		b.applyChecker(prob, tc, inputContent, output, expected, in.SPJBinContent, &r)
	case "Nonzero Exit Status":
		r.Status = model.StatusRE
		if errOut := cr.Files["error.txt"]; errOut != "" {
			r.Detail = errOut
		} else {
			r.Detail = cr.Error
		}
	case "TimeLimitExceeded":
		r.Status = model.StatusTLE
	case "MemoryLimitExceeded":
		r.Status = model.StatusMLE
	case "RuntimeError":
		r.Status = model.StatusRE
		r.Detail = cr.Error
	default:
		r.Status = model.StatusWA
		r.Detail = cr.Status
	}

	return r
}

// applyChecker runs either an SPJ or a built-in checker and updates r in place.
func (b *BatchRunner) applyChecker(
	prob *model.Problem,
	tc model.TestCaseScore,
	inputContent, output, expected, spjBinContent string,
	r *model.TestCaseResult,
) {
	if prob.SPJ && spjBinContent != "" {
		spjCopyIn := map[string]executor.CmdFile{
			"spj":        {Content: spjBinContent},
			"input.txt":  {Content: inputContent},
			"user.txt":   {Content: output},
			"answer.txt": {Content: expected},
		}
		spjResp, err := b.exec.Run(&executor.ExecRequest{
			Cmd: []executor.Cmd{{
				Args:        []string{"/bin/sh", "-c", "/box/spj /box/input.txt /box/user.txt /box/answer.txt > output.txt 2> error.txt"},
				Env:         []string{"PATH=/usr/bin:/bin"},
				CPULimit:    5_000_000_000,
				MemoryLimit: 268_435_456,
				ProcLimit:   8,
				CopyIn:      spjCopyIn,
				CopyOut:     []string{"output.txt", "error.txt"},
			}},
		})
		if err != nil {
			r.Status = model.StatusSE
			r.Detail = "SPJ run error: " + err.Error()
			return
		}
		if len(spjResp) == 0 {
			r.Status = model.StatusSE
			r.Detail = "SPJ returned no result"
			return
		}
		scr := spjResp[0]
		if scr.Status == "Accepted" && scr.ExitStatus == 0 {
			r.Status = model.StatusAC
			r.Score = tc.Score
		} else {
			r.Status = model.StatusWA
			msg := strings.TrimSpace(scr.Files["error.txt"])
			if msg == "" {
				msg = strings.TrimSpace(scr.Files["output.txt"])
			}
			if msg == "" {
				msg = "output mismatch (SPJ rejected)"
			}
			r.Detail = msg
		}
		return
	}

	chk := checker.GetChecker(prob.CheckerType, prob.FloatEpsilon)
	if ck := chk.Check(nil, []byte(expected), []byte(output)); ck.Passed {
		r.Status = model.StatusAC
		r.Score = tc.Score
	} else {
		r.Status = model.StatusWA
		r.Detail = ck.Message
	}
}

// ---------- helpers ----------

// buildRunArgs constructs the shell command arguments for running the compiled
// (or interpreted) contestant code, redirecting stdin/stdout/stderr.
func buildRunArgs(langCfg *compiler.LangConfig, cr *compiler.CompileResult) []string {
	srcName := "Main" + langCfg.Extensions[0]

	if cr != nil && cr.Success {
		if langCfg.Runtime != "" {
			rtParts := strings.Fields(langCfg.Runtime)
			for i, p := range rtParts {
				rtParts[i] = strings.ReplaceAll(p, "{{dir}}", ".")
				rtParts[i] = strings.ReplaceAll(rtParts[i], "{{exe}}", "Main")
			}
			runCmd := strings.Join(rtParts, " ") + " < input.txt > output.txt 2> error.txt"
			if cr.TarMode {
				runCmd = "tar -xf compile.tar && " + runCmd
			}
			return []string{"/bin/sh", "-c", runCmd}
		}
		runCmd := "./Main < input.txt > output.txt 2> error.txt"
		if cr.TarMode {
			runCmd = "tar -xf compile.tar && " + runCmd
		}
		return []string{"/bin/sh", "-c", runCmd}
	}

	// Interpreted (no compilation step).
	if langCfg.Runtime != "" {
		rtParts := strings.Fields(langCfg.Runtime)
		runCmd := strings.Join(rtParts, " ") + " " + srcName + " < input.txt > output.txt 2> error.txt"
		return []string{"/bin/sh", "-c", runCmd}
	}
	return []string{"/bin/sh", "-c", "./Main < input.txt > output.txt 2> error.txt"}
}

// subtaskOrderedCases returns test cases in subtask-ID order (ascending).
func subtaskOrderedCases(prob *model.Problem) []model.TestCaseScore {
	subtasks := prob.GetSubtasks()
	var ids []int
	for id := range subtasks {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	var out []model.TestCaseScore
	for _, id := range ids {
		out = append(out, subtasks[id]...)
	}
	return out
}

// loadFile reads a file from disk, returning an empty string on error.
func loadFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
