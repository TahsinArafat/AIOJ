// Package runner defines the Runner interface and shared types for the
// judging engine.  Each problem type (batch, interactive, output-only) is
// implemented as a separate Runner.
package runner

import (
	"context"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/model"
)

// RunInput bundles everything a Runner needs to judge a submission.
type RunInput struct {
	// Problem is the fully-loaded problem (with test cases, limits, etc.).
	Problem *model.Problem
	// LangCfg is the language configuration for the contestant's language.
	LangCfg *compiler.LangConfig
	// CompiledResult is the output of CompileContestant.  May be nil for
	// interpreted languages.
	CompiledResult *compiler.CompileResult
	// SourceCode is the raw contestant source (needed for interpreted
	// languages and output-only submissions).
	SourceCode string
	// SPJBinContent is the compiled SPJ binary content (empty if no SPJ).
	SPJBinContent string
	// CPULimitNs is the CPU time limit in nanoseconds.
	CPULimitNs uint64
	// MemLimitBytes is the memory limit in bytes.
	MemLimitBytes uint64
}

// RunOutput is the result of a Runner.Run call.
type RunOutput struct {
	Results     []model.TestCaseResult
	FinalStatus model.SubmissionStatus
	// Score is a normalised score in [0, 100] for partial-credit problems,
	// or an average per-test score for standard problems.
	Score   int
	MaxTime int // ms
	MaxMem  int // KB
}

// Runner judges a submission against all test cases for a single problem.
type Runner interface {
	Run(ctx context.Context, in RunInput) (RunOutput, error)
}
