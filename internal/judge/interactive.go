package judge

import (
	"context"
	"fmt"

	"github.com/tahsinarafat/aioj/internal/judge/executor"
)

// InteractiveResult holds the outcome of an interactive judging session.
type InteractiveResult struct {
	Status  string // "ac", "wa", "tle", "mle", "re", "se"
	Time    uint64
	Memory  uint64
	Message string
}

// runInteractive executes an interactor and contestant connected via bidirectional pipes.
// go-judge PipeInput connects cmd[0].stdout -> cmd[1].stdin AND cmd[1].stdout -> cmd[0].stdin.
// Convention: interactor exits 0 = AC, 1 = WA, 2+ = internal error.
func (wp *WorkerPool) runInteractive(
	ctx context.Context,
	interactorCmd []string,
	interactorCopyIn map[string]executor.CmdFile,
	contestantCmd []string,
	contestantCopyIn map[string]executor.CmdFile,
	cpuLimitNs uint64,
	memoryLimitBytes uint64,
) (*InteractiveResult, error) {
	req := &executor.ExecRequest{
		Cmd: []executor.Cmd{
			{
				Args:        interactorCmd,
				CPULimit:    30_000_000_000, // 30s for interactor
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

	results, err := wp.exec.Run(req)
	if err != nil {
		return nil, fmt.Errorf("interactive execution error: %w", err)
	}
	if len(results) < 2 {
		return nil, fmt.Errorf("expected 2 results from interactive run, got %d", len(results))
	}

	interactor := results[0]
	contestant := results[1]

	// Contestant limits take priority
	switch contestant.Status {
	case "TimeLimitExceeded":
		return &InteractiveResult{Status: "tle", Time: contestant.Time, Memory: contestant.Memory}, nil
	case "MemoryLimitExceeded":
		return &InteractiveResult{Status: "mle", Time: contestant.Time, Memory: contestant.Memory}, nil
	case "RuntimeError":
		return &InteractiveResult{Status: "re", Time: contestant.Time, Memory: contestant.Memory, Message: "runtime error"}, nil
	}

	// Interactor verdict
	switch interactor.ExitStatus {
	case 0:
		return &InteractiveResult{Status: "ac", Time: contestant.Time, Memory: contestant.Memory, Message: interactor.Error}, nil
	case 1:
		return &InteractiveResult{Status: "wa", Time: contestant.Time, Memory: contestant.Memory, Message: interactor.Error}, nil
	default:
		return &InteractiveResult{
			Status:  "se",
			Time:    contestant.Time,
			Memory:  contestant.Memory,
			Message: fmt.Sprintf("interactor exited with code %d: %s", interactor.ExitStatus, interactor.Error),
		}, nil
	}
}
