// judging engine.  Each problem type (batch, interactive, output-only) has a
// dedicated Runner.
package runner

import (
	"context"

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
