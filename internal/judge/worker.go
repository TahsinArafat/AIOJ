package judge

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/judge/compiler"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/judge/pressure"
	"github.com/tahsinarafat/aioj/internal/judge/runner"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/store"
)

// drainTimeout bounds how long Stop waits for in-flight judgments to finish.
// Beyond it the process exits anyway; the reclaimer picks the rows back up.
const drainTimeout = 30 * time.Second

// VerdictNotifier is implemented by the WebSocket manager so a finished
// verdict reaches the browser without polling. Declared here (rather than
// importing the handler package) to keep the dependency direction
// api/handler -> judge, never the reverse.
//
// A nil notifier is fine: the worker simply skips the push and the client
// falls back to its poll.
type VerdictNotifier interface {
	NotifyVerdict(submissionID, status string)
}

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

	// gate holds dispatch when the host is under memory or CPU pressure.
	// A nil gate (or disabled thresholds) means dispatch is always immediate.
	gate *pressure.PressureGate

	// notifier pushes finished verdicts to subscribed browsers. Nil is valid.
	notifier VerdictNotifier

	// langs caches the language configs loaded from langDir.  LoadLanguages
	// reads and YAML-parses every lang/*.yaml on every call — doing that per
	// submission was measurable I/O on every judgment.  The cache is populated
	// once on first use (sync.Once) and is safe for concurrent reads.  The
	// YAML files only change when a new language is added, which requires a
	// restart anyway, so there is no need to watch the directory.
	langsOnce sync.Once
	langs     map[string]*compiler.LangConfig

	// wg tracks in-flight judgments so Stop can drain instead of abandoning
	// them half-written.
	wg     sync.WaitGroup
	stopMu sync.Mutex
	closed bool
}

func NewWorkerPool(q queue.JudgeQueue, exec *executor.Client, langDir string, concurrency int, subStore store.SubmissionStore, probStore store.ProblemStore, langLimitStore store.LanguageLimitStore, balloonStore store.BalloonStore) *WorkerPool {
	return &WorkerPool{
		queue: q, exec: exec, compiler: compiler.New(exec),
		batchRunner: runner.NewBatchRunner(exec), interactiveRunner: runner.NewInteractiveRunner(exec), outputOnlyRunner: runner.NewOutputOnlyRunner(),
		langDir: langDir, sem: make(chan struct{}, concurrency), subStore: subStore, probStore: probStore,
		langLimitStore: langLimitStore, balloonStore: balloonStore,
	}
}

// WithPressureGate attaches a resource-pressure gate to the pool.  Call this
// after NewWorkerPool when pressure thresholds are configured.
func (wp *WorkerPool) WithPressureGate(g *pressure.PressureGate) *WorkerPool {
	wp.gate = g
	return wp
}

// WithNotifier attaches a verdict notifier so finished judgments are pushed to
// the browser over the existing WebSocket instead of waiting for a poll.
func (wp *WorkerPool) WithNotifier(n VerdictNotifier) *WorkerPool {
	wp.notifier = n
	return wp
}

// loadLangs returns the cached language map, loading it from disk on first
// call.  The result is shared across all goroutines in the pool.
func (wp *WorkerPool) loadLangs() map[string]*compiler.LangConfig {
	wp.langsOnce.Do(func() {
		langs, err := compiler.LoadLanguages(wp.langDir)
		if err != nil {
			slog.Error("judge: failed to load language configs", "error", err)
			langs = map[string]*compiler.LangConfig{}
		} else {
			slog.Info("judge: language configs loaded", "count", len(langs))
		}
		wp.langs = langs
	})
	return wp.langs
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

		// Hold the job in-hand and wait until the server has enough free
		// resources before dispatching it. The job has already been removed
		// from the queue, so no other worker will pick it up. If the context
		// is cancelled while we are waiting (graceful shutdown), re-enqueue
		// the job so the reclaimer can pick it back up.
		if wp.gate != nil {
			if err := wp.gate.WaitUntilReady(ctx); err != nil {
				// Context cancelled — put the submission back so it isn't lost.
				if reqErr := wp.queue.Enqueue(ctx, subID, 0); reqErr != nil {
					slog.Error("pressure: failed to re-enqueue submission after shutdown",
						"id", subID, "error", reqErr)
				}
				return
			}
		}

		select {
		case wp.sem <- struct{}{}:
			wp.wg.Add(1)
			go func(id string) {
				defer func() {
					<-wp.sem
					wp.wg.Done()
				}()
				wp.process(ctx, id)
			}(subID)
		case <-ctx.Done():
			return
		}
	}
}

// Stop waits for in-flight judgments to finish, up to drainTimeout.
//
// Without this a deploy cancelled every running judgment mid-flight: the worker
// then wrote a System Error verdict over a submission that was about to be
// judged correctly. Returning early lets the process exit and the reclaimer
// re-queue anything still marked judging.
func (wp *WorkerPool) Stop() {
	wp.stopMu.Lock()
	if wp.closed {
		wp.stopMu.Unlock()
		return
	}
	wp.closed = true
	wp.stopMu.Unlock()

	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("judge workers drained")
	case <-time.After(drainTimeout):
		slog.Warn("judge workers did not drain in time; reclaimer will recover", "timeout", drainTimeout.String())
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
		return false
	}
	// Push the verdict only after it is durably persisted: a client that
	// reconnects and re-fetches must never see a different answer than the
	// socket delivered.
	if wp.notifier != nil {
		wp.notifier.NotifyVerdict(submissionID, string(status))
	}
	return true
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
	langs := wp.loadLangs()
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
		// A cancelled context means the process is shutting down, not that the
		// submission is broken. Leave it claimed and let the reclaimer re-queue
		// it rather than recording a System Error over a verdict that was never
		// computed.
		if ctx.Err() != nil {
			slog.Warn("judgment interrupted by shutdown; leaving it for the reclaimer", "id", sub.ID)
			return
		}
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
