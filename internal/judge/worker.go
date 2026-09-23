package judge

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/judge/runner"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/store"
)

type WorkerPool struct {
	queue             queue.JudgeQueue
	exec              *executor.Client
	compiler          *compiler.Compiler
	batchRunner       *runner.BatchRunner
	interactiveRunner *runner.InteractiveRunner
	outputOnlyRunner  *runner.OutputOnlyRunner
	langDir           string
	sem               chan struct{}
	subStore          store.SubmissionStore
	probStore         store.ProblemStore
	langLimitStore    store.LanguageLimitStore
	balloonStore      store.BalloonStore
}

func NewWorkerPool(q queue.JudgeQueue, exec *executor.Client, langDir string, concurrency int, subStore store.SubmissionStore, probStore store.ProblemStore, langLimitStore store.LanguageLimitStore, balloonStore store.BalloonStore) *WorkerPool {
	return &WorkerPool{
		queue: q, exec: exec, compiler: compiler.New(exec),
		batchRunner: runner.NewBatchRunner(exec), interactiveRunner: runner.NewInteractiveRunner(exec), outputOnlyRunner: runner.NewOutputOnlyRunner(),
		langDir: langDir, sem: make(chan struct{}, concurrency), subStore: subStore, probStore: probStore,
		langLimitStore: langLimitStore, balloonStore: balloonStore,
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	for {
		subID, err := wp.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("dequeue failed", "error", err)
			time.Sleep(time.Second)
			continue
		}
		select {
		case wp.sem <- struct{}{}:
			go func(id string) {
				defer func() { <-wp.sem }()
				wp.process(ctx, id)
			}(subID)
		case <-ctx.Done():
			return
		}
	}
}

func (wp *WorkerPool) process(ctx context.Context, submissionID string) {
	claimToken := uuid.NewString()
	claimed, err := wp.subStore.ClaimPending(ctx, submissionID, claimToken)
	if err != nil {
		slog.Error("claim submission failed", "id", submissionID, "error", err)
		return
	}
	if !claimed {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("judge panic", "id", submissionID, "panic", r)
			wp.finish(context.Background(), submissionID, claimToken, model.StatusSE, 0, 0, 0, fmt.Sprintf("panic: %v", r), nil)
		}
	}()
	wp.judge(ctx, submissionID, claimToken)
}

func (wp *WorkerPool) finish(ctx context.Context, submissionID, claimToken string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) bool {
	applied, err := wp.subStore.UpdateResultClaimed(ctx, submissionID, claimToken, status, score, timeUsed, memoryUsed, compileOutput, results)
	if err != nil {
		slog.Error("persist verdict failed", "id", submissionID, "error", err)
		return false
	}
	if !applied {
		slog.Warn("discarded superseded verdict", "id", submissionID, "status", status)
	}
	return applied
}

func (wp *WorkerPool) judge(ctx context.Context, submissionID, claimToken string) {
	sub, err := wp.subStore.GetByID(ctx, submissionID)
	if err != nil || sub == nil {
		wp.finish(context.Background(), submissionID, claimToken, model.StatusSE, 0, 0, 0, fmt.Sprintf("failed to load submission: %v", err), nil)
		return
	}
	prob, err := wp.probStore.GetByID(ctx, sub.ProblemID)
	if err != nil || prob == nil {
		wp.finish(ctx, submissionID, claimToken, model.StatusSE, 0, 0, 0, "problem not found", nil)
		return
	}
	if wp.langLimitStore != nil {
		if limits, err := wp.langLimitStore.GetByProblem(ctx, prob.ID); err == nil {
			for _, ll := range limits {
				if ll != nil {
					prob.LanguageLimits = append(prob.LanguageLimits, *ll)
				}
			}
		}
	}
	if sub.SubmissionType == model.SubmissionTypeOutput {
		wp.runAndFinalize(ctx, sub, claimToken, prob, wp.outputOnlyRunner, runner.RunInput{Problem: prob, SourceCode: sub.SourceCode}, "")
		return
	}
	langs, _ := compiler.LoadLanguages(wp.langDir)
	cfg := langs[sub.Language]
	if cfg == nil {
		wp.finish(ctx, submissionID, claimToken, model.StatusCE, 0, 0, 0, "unsupported language: "+sub.Language, nil)
		return
	}
	if prob.Interactive {
		wp.judgeInteractiveSub(ctx, sub, claimToken, prob, cfg, langs)
		return
	}

	var spjFile executor.CmdFile
	if prob.SPJ && prob.SPJSourceCode != "" {
		spjFile, err = wp.compiler.CompileSPJ(ctx, prob.SPJSourceCode, prob.SPJLanguage, langs)
		if err != nil {
			wp.finish(ctx, submissionID, claimToken, model.StatusSE, 0, 0, 0, err.Error(), nil)
			return
		}
	}
	compiled, err := wp.compiler.CompileContestant(ctx, sub.SourceCode, cfg)
	if err != nil {
		wp.finish(ctx, submissionID, claimToken, model.StatusSE, 0, 0, 0, "compilation failed: "+err.Error(), nil)
		return
	}
	if !compiled.Success {
		wp.finish(ctx, submissionID, claimToken, model.StatusCE, 0, 0, 0, compiled.Output, nil)
		wp.probStore.UpdateCounts(ctx, prob.ID, 1, 0)
		return
	}
	timeLimitMs, memoryLimitKB := getEffectiveLimits(prob, sub.Language)
	runIn := runner.RunInput{
		Problem: prob, LangCfg: cfg, CompiledResult: compiled, SourceCode: sub.SourceCode, SPJFile: spjFile,
		CPULimitNs:    uint64(float64(timeLimitMs)*cfg.TimeLimitMultiplier) * 1_000_000,
		MemLimitBytes: uint64(float64(memoryLimitKB)*cfg.MemoryLimitMultiplier) * 1024,
	}
	wp.runAndFinalize(ctx, sub, claimToken, prob, wp.batchRunner, runIn, compiled.Output)
}

func (wp *WorkerPool) judgeInteractiveSub(ctx context.Context, sub *model.Submission, claimToken string, prob *model.Problem, cfg *compiler.LangConfig, langs map[string]*compiler.LangConfig) {
	compiled, err := wp.compiler.CompileContestant(ctx, sub.SourceCode, cfg)
	if err != nil {
		wp.finish(ctx, sub.ID, claimToken, model.StatusSE, 0, 0, 0, err.Error(), nil)
		return
	}
	if !compiled.Success {
		wp.finish(ctx, sub.ID, claimToken, model.StatusCE, 0, 0, 0, compiled.Output, nil)
		return
	}
	interactorFile, err := wp.compiler.CompileInteractor(ctx, prob.InteractorSourceCode, prob.InteractorLanguage, langs)
	if err != nil {
		wp.finish(ctx, sub.ID, claimToken, model.StatusSE, 0, 0, 0, err.Error(), nil)
		return
	}
	timeLimitMs, memoryLimitKB := getEffectiveLimits(prob, sub.Language)
	runIn := runner.RunInput{
		Problem: prob, LangCfg: cfg, CompiledResult: compiled, SourceCode: sub.SourceCode, InteractorFile: interactorFile,
		CPULimitNs:    uint64(float64(timeLimitMs)*cfg.TimeLimitMultiplier) * 1_000_000,
		MemLimitBytes: uint64(float64(memoryLimitKB)*cfg.MemoryLimitMultiplier) * 1024,
	}
	wp.runAndFinalize(ctx, sub, claimToken, prob, wp.interactiveRunner, runIn, "")
}

func (wp *WorkerPool) runAndFinalize(ctx context.Context, sub *model.Submission, claimToken string, prob *model.Problem, r runner.Runner, in runner.RunInput, compileOutput string) {
	out, err := r.Run(ctx, in)
	if err != nil {
		wp.finish(ctx, sub.ID, claimToken, model.StatusSE, 0, 0, 0, err.Error(), nil)
		return
	}
	if !wp.finish(ctx, sub.ID, claimToken, out.FinalStatus, out.Score, out.MaxTime, out.MaxMem, compileOutput, out.Results) {
		return
	}
	if out.FinalStatus == model.StatusAC && sub.ContestID != "" {
		_ = wp.balloonStore.CreateRequest(ctx, sub.ContestID, sub.ID, sub.UserID, sub.ProblemID)
	}
	wp.probStore.UpdateCounts(ctx, prob.ID, 1, boolToInt(out.FinalStatus == model.StatusAC))
	slog.Info("judged", "id", sub.ID, "verdict", out.FinalStatus)
}

func getEffectiveLimits(prob *model.Problem, language string) (timeLimitMs int, memoryLimitKB int) {
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

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
