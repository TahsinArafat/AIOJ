package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/tahsinarafat/aioj/internal/judge/checker"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/model"
)

// InteractiveRunner judges interactive problems using bidirectional pipes
// between the contestant process and the interactor.
type InteractiveRunner struct {
	exec   *executor.Client
	scorer *Scorer
}

// NewInteractiveRunner returns an InteractiveRunner backed by the given executor.
func NewInteractiveRunner(exec *executor.Client) *InteractiveRunner {
	return &InteractiveRunner{exec: exec, scorer: NewScorer()}
}

// Run implements Runner.
// RunInput.CompiledResult must be the compiled contestant binary.
// The interactor binary content is stored in RunInput.SPJBinContent
// (the field is repurposed when running interactive problems).
func (ir *InteractiveRunner) Run(ctx context.Context, in RunInput) (RunOutput, error) {
	prob := in.Problem
	cfg := in.LangCfg
	srcName := "Main" + cfg.Extensions[0]

	compiledBinContent := ""
	if in.CompiledResult != nil && in.CompiledResult.Success {
		if bin, ok := in.CompiledResult.Files["Main"]; ok {
			compiledBinContent = bin.Content
		}
	}

	interBinContent := in.SPJBinContent // convention: interactor binary stored here

	results := make([]model.TestCaseResult, 0, len(prob.TestCaseScore))
	finalStatus := model.StatusAC
	maxTime, maxMem, totalScore := 0, 0, 0

	for _, tc := range prob.TestCaseScore {
		inputContent := loadFile(filepath.Join(prob.TestdataPath, tc.InputName))

		interRunCopyIn := map[string]executor.CmdFile{
			"interactor": {Content: interBinContent},
		}
		if inputContent != "" {
			interRunCopyIn["input.txt"] = executor.CmdFile{Content: inputContent}
		}

		contestantRunCopyIn := map[string]executor.CmdFile{}
		var contestantArgs []string
		if cfg.Runtime != "" {
			rtParts := strings.Fields(cfg.Runtime)
			contestantArgs = append(rtParts, "/box/"+srcName)
			contestantRunCopyIn[srcName] = executor.CmdFile{Content: in.SourceCode}
		} else {
			contestantArgs = []string{"/box/Main"}
			contestantRunCopyIn["Main"] = executor.CmdFile{Content: compiledBinContent}
		}

		interArgs := []string{"/box/interactor"}
		if inputContent != "" {
			interArgs = append(interArgs, "/box/input.txt")
		}

		iResult, err := runInteractivePair(ctx, ir.exec,
			interArgs, interRunCopyIn,
			contestantArgs, contestantRunCopyIn,
			in.CPULimitNs, in.MemLimitBytes)
		if err != nil {
			results = append(results, model.TestCaseResult{
				CaseName: tc.InputName,
				Status:   model.StatusSE,
				Detail:   err.Error(),
			})
			if finalStatus == model.StatusAC {
				finalStatus = model.StatusSE
			}
			continue
		}

		r := model.TestCaseResult{
			CaseName: tc.InputName,
			Time:     int(iResult.Time / 1_000_000), // ns → ms
			Memory:   int(iResult.Memory / 1024),    // bytes → KB
		}
		switch iResult.Status {
		case "ac":
			r.Status = model.StatusAC
			r.Score = tc.Score
			totalScore += tc.Score
		case "wa":
			r.Status = model.StatusWA
			r.Detail = iResult.Message
		case "tle":
			r.Status = model.StatusTLE
		case "mle":
			r.Status = model.StatusMLE
		case "re":
			r.Status = model.StatusRE
			r.Detail = iResult.Message
		default:
			r.Status = model.StatusSE
			r.Detail = iResult.Message
		}

		results = append(results, r)
		if r.Time > maxTime {
			maxTime = r.Time
		}
		if r.Memory > maxMem {
			maxMem = r.Memory
		}
		if r.Status != model.StatusAC && finalStatus == model.StatusAC {
			finalStatus = r.Status
		}
	}

	avgScore := 0
	if len(prob.TestCaseScore) > 0 {
		avgScore = totalScore / len(prob.TestCaseScore)
	}

	return RunOutput{
		Results:     results,
		FinalStatus: finalStatus,
		Score:       avgScore,
		MaxTime:     maxTime,
		MaxMem:      maxMem,
	}, nil
}

// ─── OutputOnlyRunner ────────────────────────────────────────────────────────

// OutputOnlyRunner judges output-only problems: the submission's SourceCode
// field IS the contestant's output, compared against expected outputs.
type OutputOnlyRunner struct{}

// NewOutputOnlyRunner returns an OutputOnlyRunner.
func NewOutputOnlyRunner() *OutputOnlyRunner { return &OutputOnlyRunner{} }

// Run implements Runner.
func (or *OutputOnlyRunner) Run(ctx context.Context, in RunInput) (RunOutput, error) {
	prob := in.Problem
	contestantOutput := []byte(in.SourceCode)

	chk := checker.GetChecker(prob.CheckerType, prob.FloatEpsilon)

	var results []model.TestCaseResult
	totalScore, maxScore := 0, 0
	allPassed := true

	for _, tc := range prob.TestCaseScore {
		maxScore += tc.Score

		expectedOutput, err := os.ReadFile(filepath.Join(prob.TestdataPath, tc.OutputName))
		if err != nil {
			results = append(results, model.TestCaseResult{
				CaseName: tc.InputName,
				Status:   model.StatusSE,
				Detail:   "failed to load expected output: " + err.Error(),
			})
			allPassed = false
			continue
		}

		result := chk.Check(nil, expectedOutput, contestantOutput)
		score := 0
		status := model.StatusWA
		if result.Passed {
			score = tc.Score
			status = model.StatusAC
		} else {
			allPassed = false
		}
		totalScore += score

		results = append(results, model.TestCaseResult{
			CaseName: tc.InputName,
			Status:   status,
			Score:    score,
			Detail:   result.Message,
		})
	}

	finalStatus := model.StatusWA
	if allPassed {
		finalStatus = model.StatusAC
	}

	percentageScore := 0
	if maxScore > 0 {
		percentageScore = (totalScore * 100) / maxScore
	}

	return RunOutput{
		Results:     results,
		FinalStatus: finalStatus,
		Score:       percentageScore,
	}, nil
}

// ─── Internal interactive helpers ────────────────────────────────────────────

type interactiveResult struct {
	Status  string
	Time    uint64
	Memory  uint64
	Message string
}

func runInteractivePair(
	ctx context.Context,
	exec *executor.Client,
	interactorCmd []string,
	interactorCopyIn map[string]executor.CmdFile,
	contestantCmd []string,
	contestantCopyIn map[string]executor.CmdFile,
	cpuLimitNs uint64,
	memoryLimitBytes uint64,
) (*interactiveResult, error) {
	req := &executor.ExecRequest{
		Cmd: []executor.Cmd{
			{
				Args:        interactorCmd,
				CPULimit:    30_000_000_000,
				MemoryLimit: 512 * 1024 * 1024,
				ProcLimit:   64,
				CopyIn:      interactorCopyIn,
			},
			{
				Args:        contestantCmd,
				CPULimit:    cpuLimitNs,
				MemoryLimit: memoryLimitBytes,
				ProcLimit:   64,
				CopyIn:      contestantCopyIn,
			},
		},
		PipeInput: true,
	}

	results, err := exec.Run(req)
	if err != nil {
		return nil, err
	}
	if len(results) < 2 {
		return nil, &interactiveRunError{}
	}

	interactor := results[0]
	contestant := results[1]

	switch contestant.Status {
	case "TimeLimitExceeded":
		return &interactiveResult{Status: "tle", Time: contestant.Time, Memory: contestant.Memory}, nil
	case "MemoryLimitExceeded":
		return &interactiveResult{Status: "mle", Time: contestant.Time, Memory: contestant.Memory}, nil
	case "RuntimeError":
		return &interactiveResult{Status: "re", Time: contestant.Time, Memory: contestant.Memory, Message: "runtime error"}, nil
	}

	switch interactor.ExitStatus {
	case 0:
		return &interactiveResult{Status: "ac", Time: contestant.Time, Memory: contestant.Memory, Message: interactor.Error}, nil
	case 1:
		return &interactiveResult{Status: "wa", Time: contestant.Time, Memory: contestant.Memory, Message: interactor.Error}, nil
	default:
		return &interactiveResult{
			Status:  "se",
			Time:    contestant.Time,
			Memory:  contestant.Memory,
			Message: "interactor exited with non-standard code",
		}, nil
	}
}

type interactiveRunError struct{}

func (e *interactiveRunError) Error() string {
	return "expected 2 results from interactive run"
}
