package judge

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/judge/runner"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/store"
)

// WorkerPool dequeues submission IDs and dispatches them to the appropriate
// Runner (batch, interactive, or output-only).  All judging logic lives in the
// runner and compiler sub-packages.
type WorkerPool struct {
	queue               queue.JudgeQueue
	exec                *executor.Client
	compiler            *compiler.Compiler
	batchRunner         *runner.BatchRunner
	interactiveRunner   *runner.InteractiveRunner
	outputOnlyRunner    *runner.OutputOnlyRunner
	langDir             string
	sem                 chan struct{}
	subStore            store.SubmissionStore
	probStore           store.ProblemStore
	langLimitStore      store.LanguageLimitStore
	balloonStore        store.BalloonStore
}

func NewWorkerPool(
	q queue.JudgeQueue,
	exec *executor.Client,
	langDir string,
	concurrency int,
	subStore store.SubmissionStore,
	probStore store.ProblemStore,
	langLimitStore store.LanguageLimitStore,
	balloonStore store.BalloonStore,
) *WorkerPool {
	return &WorkerPool{
		queue:             q,
		exec:              exec,
		compiler:          compiler.New(exec),
		batchRunner:       runner.NewBatchRunner(exec),
		interactiveRunner: runner.NewInteractiveRunner(exec),
		outputOnlyRunner:  runner.NewOutputOnlyRunner(),
		langDir:           langDir,
		sem:               make(chan struct{}, concurrency),
		subStore:          subStore,
		probStore:         probStore,
		langLimitStore:    langLimitStore,
		balloonStore:      balloonStore,
	}
}

// Start consumes from the queue and dispatches goroutines bounded by sem.
func (wp *WorkerPool) Start(ctx context.Context) {
	for {
		subID, err := wp.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("dequeue failed", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}
		select {
		case wp.sem <- struct{}{}:
			go func(id string) {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("judge panic", "id", id, "panic", r)
						wp.subStore.UpdateResult(context.Background(), id, model.StatusSE, 0, 0, 0, fmt.Sprintf("panic: %v", r), nil)
					}
					<-wp.sem
				}()
				wp.judge(ctx, id)
			}(subID)
		case <-ctx.Done():
			return
		}
	}
}

// judge loads the submission + problem, chooses a runner, and persists the result.
func (wp *WorkerPool) judge(ctx context.Context, submissionID string) {
	slog.Info("judging", "id", submissionID)

	sub, err := wp.subStore.GetByID(context.Background(), submissionID)
	if err != nil || sub == nil {
		slog.Error("load sub failed", "id", submissionID, "error", err)
		wp.subStore.UpdateResult(context.Background(), submissionID, model.StatusSE, 0, 0, 0,
			fmt.Sprintf("failed to load submission: %v", err), nil)
		return
	}

	wp.subStore.UpdateStatus(context.Background(), submissionID, model.StatusJudging)

	prob, err := wp.probStore.GetByID(ctx, sub.ProblemID)
	if err != nil || prob == nil {
		wp.subStore.UpdateResult(ctx, submissionID, model.StatusSE, 0, 0, 0, "problem not found", nil)
		return
	}

	// Load per-language limits.
	if wp.langLimitStore != nil {
		if limits, err := wp.langLimitStore.GetByProblem(ctx, prob.ID); err == nil {
			for _, ll := range limits {
				prob.LanguageLimits = append(prob.LanguageLimits, *ll)
			}
		}
	}

	// ── Route by problem / submission type ───────────────────────────────────

	if sub.SubmissionType == model.SubmissionTypeOutput {
		wp.runAndFinalize(ctx, sub, prob, wp.outputOnlyRunner, runner.RunInput{
			Problem:    prob,
			SourceCode: sub.SourceCode,
		}, "")
		return
	}

	langs, _ := compiler.LoadLanguages(wp.langDir)
	cfg := langs[sub.Language]
	if cfg == nil {
		wp.subStore.UpdateResult(ctx, submissionID, model.StatusCE, 0, 0, 0, "unsupported language: "+sub.Language, nil)
		return
	}

	if prob.Interactive {
		wp.judgeInteractiveSub(ctx, sub, prob, cfg, langs)
		return
	}

	// ── Standard batch judging ───────────────────────────────────────────────

	// Compile SPJ (if present).
	spjBinContent := ""
	if prob.SPJ && prob.SPJSourceCode != "" {
		slog.Info("compiling SPJ", "lang", prob.SPJLanguage)
		bin, err := wp.compiler.CompileSPJ(ctx, prob.SPJSourceCode, prob.SPJLanguage, langs)
		if err != nil {
			wp.subStore.UpdateResult(ctx, submissionID, model.StatusSE, 0, 0, 0, err.Error(), nil)
			return
		}
		spjBinContent = bin
	}

	// Compile contestant.
	compiledResult, err := wp.compiler.CompileContestant(ctx, sub.SourceCode, cfg)
	if err != nil {
		wp.subStore.UpdateResult(ctx, submissionID, model.StatusSE, 0, 0, 0, "compilation failed: "+err.Error(), nil)
		return
	}
	if !compiledResult.Success {
		wp.subStore.UpdateResult(ctx, submissionID, model.StatusCE, 0, 0, 0, compiledResult.Output, nil)
		wp.probStore.UpdateCounts(ctx, prob.ID, 1, 0)
		return
	}

	timeLimitMs, memLimitKB := getEffectiveLimits(prob, sub.Language)
	cpuNs := uint64(float64(timeLimitMs)*cfg.TimeLimitMultiplier) * 1_000_000
	memBytes := uint64(float64(memLimitKB)*cfg.MemoryLimitMultiplier) * 1024

	runIn := runner.RunInput{
		Problem:        prob,
		LangCfg:        cfg,
		CompiledResult: compiledResult,
		SourceCode:     sub.SourceCode,
		SPJBinContent:  spjBinContent,
		CPULimitNs:     cpuNs,
		MemLimitBytes:  memBytes,
	}
	wp.runAndFinalize(ctx, sub, prob, wp.batchRunner, runIn, compiledResult.Output)
}

// judgeInteractiveSub compiles both binaries then delegates to InteractiveRunner.
func (wp *WorkerPool) judgeInteractiveSub(
	ctx context.Context,
	sub *model.Submission,
	prob *model.Problem,
	cfg *compiler.LangConfig,
	langs map[string]*compiler.LangConfig,
) {
	slog.Info("compiling contestant (interactive)", "lang", sub.Language)
	compiledResult, err := wp.compiler.CompileContestant(ctx, sub.SourceCode, cfg)
	if err != nil {
		wp.subStore.UpdateResult(ctx, sub.ID, model.StatusCE, 0, 0, 0, err.Error(), nil)
		return
	}
	if !compiledResult.Success {
		wp.subStore.UpdateResult(ctx, sub.ID, model.StatusCE, 0, 0, 0, compiledResult.Output, nil)
		return
	}

	slog.Info("compiling interactor", "lang", prob.InteractorLanguage)
	interBinContent, err := wp.compiler.CompileInteractor(ctx, prob.InteractorSourceCode, prob.InteractorLanguage, langs)
	if err != nil {
		wp.subStore.UpdateResult(ctx, sub.ID, model.StatusSE, 0, 0, 0, err.Error(), nil)
		return
	}

	timeLimitMs, memLimitKB := getEffectiveLimits(prob, sub.Language)
	cpuNs := uint64(float64(timeLimitMs)*float64(cfg.TimeLimitMultiplier)) * 1_000_000
	memBytes := uint64(float64(memLimitKB)*float64(cfg.MemoryLimitMultiplier)) * 1024

	runIn := runner.RunInput{
		Problem:        prob,
		LangCfg:        cfg,
		CompiledResult: compiledResult,
		SourceCode:     sub.SourceCode,
		SPJBinContent:  interBinContent, // reused field for interactor binary
		CPULimitNs:     cpuNs,
		MemLimitBytes:  memBytes,
	}
	wp.runAndFinalize(ctx, sub, prob, wp.interactiveRunner, runIn, "")
}

// runAndFinalize executes a runner and persists the result.
func (wp *WorkerPool) runAndFinalize(
	ctx context.Context,
	sub *model.Submission,
	prob *model.Problem,
	r runner.Runner,
	in runner.RunInput,
	compileOutput string,
) {
	out, err := r.Run(ctx, in)
	if err != nil {
		wp.subStore.UpdateResult(ctx, sub.ID, model.StatusSE, 0, 0, 0, err.Error(), nil)
		return
	}

	wp.subStore.UpdateResult(ctx, sub.ID, out.FinalStatus, out.Score, out.MaxTime, out.MaxMem, compileOutput, out.Results)
	if out.FinalStatus == model.StatusAC && sub.ContestID != "" {
		_ = wp.balloonStore.CreateRequest(ctx, sub.ContestID, sub.ID, sub.UserID, sub.ProblemID)
	}
	wp.probStore.UpdateCounts(ctx, prob.ID, 1, boolToInt(out.FinalStatus == model.StatusAC))
	slog.Info("judged", "id", sub.ID, "verdict", out.FinalStatus)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// getEffectiveLimits returns per-language overrides if present, else problem defaults.
func getEffectiveLimits(prob *model.Problem, language string) (timeLimitMs int, memoryLimitKB int) {
	timeLimitMs = prob.TimeLimit
	memoryLimitKB = prob.MemoryLimit
	for _, ll := range prob.LanguageLimits {
		if ll.LanguageID == language {
			if ll.TimeLimitMs != nil {
				timeLimitMs = *ll.TimeLimitMs
			}
			if ll.MemoryLimitKB != nil {
				memoryLimitKB = *ll.MemoryLimitKB
			}
			break
		}
	}
	return
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
