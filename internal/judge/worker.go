package judge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
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

// megabyte is used to render memory figures in the verdict text an admin reads.
const megabyte uint64 = 1 << 20

// DefaultJudgmentTimeout is the overall budget for one submission, across every
// test case and every sandbox call it needs.
//
// Why a whole-job deadline is needed when each sandbox call already has a 60s
// HTTP timeout: that timeout bounds one call, not the job. A batch runs one call
// per test case and does not exit early on a failure — correct, because a
// partial score is the point of subtask scoring — so a slow sandbox multiplies
// the per-call timeout by the case count. With 20 cases that is up to 20 minutes
// inside one slot, and the pool has only concurrency × workers slots (4 by
// default). Four such submissions stall the whole judge while both containers
// still look healthy, because the process is parked in a legitimate wait rather
// than crashed. Restarting the containers "fixed" it by releasing the parked
// slots, which is why the symptom looked like a container fault.
//
// The value is deliberately generous: a cold Rust or Java compile on a loaded
// host takes minutes, not seconds, and a real judgment must not be cut short.
// This is a backstop for a wedge, not a performance limit.
const DefaultJudgmentTimeout = 10 * time.Minute

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
	subStore          submissionJudgeStore
	probStore         judgeProblemStore
	langLimitStore    store.LanguageLimitStore
	balloonStore      store.BalloonStore

	// gate holds dispatch when the host is under memory or CPU pressure.
	// A nil gate (or disabled thresholds) means dispatch is always immediate.
	gate *pressure.PressureGate

	// reservedBytes is memory committed to judgments that have been admitted but
	// whose sandbox has not allocated yet. The gate subtracts it from the free
	// reading so two large judgments are not admitted on the strength of one
	// pool of memory. Atomic because every slot goroutine updates it.
	reservedBytes atomic.Int64

	// maxJudgmentBytes is the largest per-judgment memory requirement this host
	// will attempt. A submission above it is failed immediately instead of being
	// queued: waiting cannot help, because no amount of freed memory will ever
	// satisfy a requirement larger than the machine's budget.
	maxJudgmentBytes uint64

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

	// judgmentTimeout caps a single submission end to end. Zero means the
	// package default; see DefaultJudgmentTimeout for why it exists.
	judgmentTimeout time.Duration

	// health records loop progress for the liveness probe. Never nil in a pool
	// built by NewWorkerPool, but nil-checked so a hand-built one in a test does
	// not panic on the judge path.
	health *HealthState

	// wg tracks in-flight judgments so Stop can drain instead of abandoning
	// them half-written.
	wg     sync.WaitGroup
	stopMu sync.Mutex
	closed bool
}

func NewWorkerPool(q queue.JudgeQueue, exec *executor.Client, langDir string, concurrency int, subStore submissionJudgeStore, probStore store.ProblemStore, langLimitStore store.LanguageLimitStore, balloonStore store.BalloonStore) *WorkerPool {
	return &WorkerPool{
		queue: q, exec: exec, compiler: compiler.New(exec),
		batchRunner: runner.NewBatchRunner(exec), interactiveRunner: runner.NewInteractiveRunner(exec), outputOnlyRunner: runner.NewOutputOnlyRunner(),
		langDir: langDir, sem: make(chan struct{}, concurrency), subStore: subStore, probStore: probStore,
		langLimitStore: langLimitStore, balloonStore: balloonStore,
		judgmentTimeout: DefaultJudgmentTimeout,
		health:          NewHealthState(),
	}
}

// WithJudgmentTimeout overrides the per-submission budget. Non-positive values
// keep the default, so a zero-valued config cannot mean "no limit".
func (wp *WorkerPool) WithJudgmentTimeout(d time.Duration) *WorkerPool {
	if d > 0 {
		wp.judgmentTimeout = d
	}
	return wp
}

// WithPressureGate attaches a resource-pressure gate to the pool.  Call this
// after NewWorkerPool when pressure thresholds are configured.
func (wp *WorkerPool) WithPressureGate(g *pressure.PressureGate) *WorkerPool {
	wp.gate = g
	return wp
}

// WithMaxJudgmentMemory sets the largest per-judgment memory requirement the
// host will attempt, in bytes. Non-zero is what makes an unsatisfiable
// submission fail fast instead of waiting in the queue forever; zero disables
// the check, which is the right behaviour only when the gate is also disabled.
func (wp *WorkerPool) WithMaxJudgmentMemory(limit uint64) *WorkerPool {
	wp.maxJudgmentBytes = limit
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

		// Resolve how much RAM this judgment will ask the sandbox for *before*
		// waiting on resources. The gate cannot judge "is there room" without
		// knowing the size of what is coming; without this it only sees a flat
		// free-memory floor and admits a 2 GB judgment into 600 MB.
		//
		// Skipped entirely when neither feature is configured, so a deployment
		// with the gate off pays no extra reads per submission.
		//
		// An error here is not fatal: process() re-reads the submission and
		// writes the real verdict, so a transient read failure falls back to the
		// gate's flat floor rather than failing the submission twice.
		var needBytes uint64
		if wp.gate != nil || wp.maxJudgmentBytes > 0 {
			var err error
			needBytes, err = wp.requiredMemoryBytes(ctx, subID)
			if err != nil {
				slog.Warn("could not resolve judgment memory requirement; admitting on the flat floor",
					"id", subID, "error", err)
				needBytes = 0
			}
		}

		// A requirement larger than this host's budget can never be satisfied, so
		// waiting would park the submission in the queue forever. Fail it with a
		// message an admin can act on instead.
		if wp.maxJudgmentBytes > 0 && needBytes > wp.maxJudgmentBytes {
			wp.rejectOversized(ctx, subID, needBytes)
			continue
		}

		// Hold the job in-hand and wait until the server has enough free
		// resources before dispatching it. The job has already been removed
		// from the queue, so no other worker will pick it up. If the context
		// is cancelled while we are waiting (graceful shutdown), re-enqueue
		// the job so the reclaimer can pick it back up.
		if wp.gate != nil {
			if err := wp.gate.Admit(ctx, needBytes, uint64(wp.reservedBytes.Load())); err != nil {
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
			wp.reservedBytes.Add(int64(needBytes))
			go func(id string, reserved uint64) {
				defer func() {
					wp.reservedBytes.Add(-int64(reserved))
					<-wp.sem
					wp.wg.Done()
				}()
				wp.process(ctx, id)
			}(subID, needBytes)
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
	// Registered before the deadline is armed so the health probe sees the job
	// even if arming or judging is where the worker wedges. Our own deadline
	// normally ends this, so a job that outlives it means the enforcement path
	// never ran — the condition the probe exists to detect and restart.
	if wp.health != nil {
		wp.health.BeginJob(submissionID)
		defer wp.health.EndJob(submissionID)
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("judge panic", "id", submissionID, "panic", r)
			wp.finish(context.Background(), submissionID, claimToken, model.StatusSE, 0, 0, 0, fmt.Sprintf("panic: %v", r), nil)
		}
	}()
	// Bound the whole judgment, not just each sandbox call. The derived context
	// stays distinguishable from the parent: a shutdown cancels it with
	// context.Canceled, while this deadline expires it with
	// context.DeadlineExceeded, and those two need opposite handling — see
	// runAndFinalize.
	//
	// The zero check is not defensive padding: WithTimeout(ctx, 0) is already
	// expired, so a pool built as a struct literal — as the pool's own tests do —
	// would fail every submission instantly with a timeout it never intended.
	timeout := wp.judgmentTimeout
	if timeout <= 0 {
		timeout = DefaultJudgmentTimeout
	}
	judgeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	wp.judge(judgeCtx, submissionID, claimToken)
}

// finish persists a verdict.
//
// When the passed context is already done the verdict is still written, using a
// detached context: by the time this is called the answer has been computed, and
// an expired judgment deadline must not be the reason it is thrown away. Losing
// it would leave the row 'judging' until the reclaimer's 15-minute sweep — the
// stall this whole change exists to remove. Shutdown deliberately does not reach
// here at all; runAndFinalize returns early so the reclaimer re-runs the job
// instead of recording a verdict nobody computed.
func (wp *WorkerPool) finish(ctx context.Context, submissionID, claimToken string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) bool {
	if ctx.Err() != nil {
		ctx = context.Background()
	}
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
		// Three ways to get here, and they need three different answers.
		//
		// 1. The judgment deadline expired. The sandbox is wedged or the problem
		//    is pathological; either way the verdict is "system error" and the
		//    slot must be released now. Writing it is the whole point — leaving
		//    the row claimed would hold the slot until the reclaimer's sweep.
		// 2. A submission-level error (a failed sandbox call, an unreadable test
		//    file). A normal System Error verdict.
		// 3. Shutdown. Nothing was computed, so recording anything would be a
		//    lie. Leave it claimed and let the reclaimer re-queue it.
		//
		// Shutdown is checked first and by Cause: with a deadline armed, a
		// shutdown cancels the derived context too, and reading only ctx.Err()
		// cannot tell "we are stopping" from "we ran out of time".
		if cause := context.Cause(ctx); cause != nil {
			if errors.Is(cause, context.DeadlineExceeded) {
				// Report the value actually enforced, not the raw field: a
				// structurally-built pool leaves the field zero, and "exceeded the
				// 0s time budget" would be nonsense in a verdict an admin reads.
				effective := wp.effectiveJudgmentTimeout()
				slog.Error("judgment exceeded its time budget; failing it to release the slot",
					"id", sub.ID, "timeout", effective.String(), "error", err)
				wp.finish(context.Background(), sub.ID, claimToken, model.StatusSE, 0, 0, 0,
					fmt.Sprintf("judgment exceeded the %s time budget: %v", effective, err), nil)
				return
			}
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
	if out.FinalStatus == model.StatusSE {
		// The verdict row keeps this text for admins and the problem's setters;
		// the log keeps it for whoever is watching the worker.
		slog.Error("judgment failed with system error", "id", sub.ID, "problem", prob.ID,
			"contest", sub.ContestID, "error", compileOutput)
		return
	}
	slog.Info("judged", "id", sub.ID, "verdict", out.FinalStatus)
}

// requiredMemoryBytes resolves what the sandbox will be asked to allocate for
// this submission, in bytes.
//
// It mirrors the conversion the judge path applies (see judge()): the problem's
// KB limit, replaced by any per-language override, then scaled by that
// language's memory multiplier. The multiplier is why this cannot simply read
// the problem row — Java's is 2.0, so a 2 GB problem needs a 4 GB budget.
//
// A submission whose language is unknown resolves to 0: judge() fails it as a
// compile error before any sandbox runs, so there is nothing to reserve.
func (wp *WorkerPool) requiredMemoryBytes(ctx context.Context, submissionID string) (uint64, error) {
	sub, err := wp.subStore.GetByID(ctx, submissionID)
	if err != nil {
		return 0, err
	}
	if sub == nil {
		return 0, fmt.Errorf("submission %s not found", submissionID)
	}
	// Output-only submissions are checked by comparing files; they never enter a
	// sandbox, so reserving the problem's memory limit for one would idle a slot's
	// worth of RAM for nothing.
	if sub.SubmissionType == model.SubmissionTypeOutput {
		return 0, nil
	}
	prob, err := wp.probStore.GetByID(ctx, sub.ProblemID)
	if err != nil {
		return 0, err
	}
	if prob == nil {
		return 0, fmt.Errorf("problem %s not found", sub.ProblemID)
	}
	// The judge path folds these in before computing the limit; do the same here
	// or the reservation is computed from a limit that will not be applied.
	if wp.langLimitStore != nil {
		if limits, lerr := wp.langLimitStore.GetByProblem(ctx, prob.ID); lerr == nil {
			for _, ll := range limits {
				if ll != nil {
					prob.LanguageLimits = append(prob.LanguageLimits, *ll)
				}
			}
		}
	}
	cfg := wp.loadLangs()[sub.Language]
	if cfg == nil {
		return 0, nil
	}
	_, memoryLimitKB := getEffectiveLimits(prob, sub.Language)
	return uint64(float64(memoryLimitKB)*cfg.MemoryLimitMultiplier) * 1024, nil
}

// rejectOversized fails a submission whose memory requirement exceeds this
// host's per-judgment budget.
//
// This is the alternative to blocking in the gate: a requirement larger than the
// budget can never be met, so waiting would park the row in the queue forever
// with the user staring at "Judging...". The verdict names the three ways an
// admin can resolve it.
func (wp *WorkerPool) rejectOversized(ctx context.Context, submissionID string, needBytes uint64) {
	claimToken := uuid.NewString()
	claimed, err := wp.subStore.ClaimPending(ctx, submissionID, claimToken)
	if err != nil {
		slog.Error("claim oversized submission failed", "id", submissionID, "error", err)
		return
	}
	if !claimed {
		return
	}
	slog.Error("rejecting submission above the judge memory budget",
		"id", submissionID, "need_mb", needBytes/megabyte, "budget_mb", wp.maxJudgmentBytes/megabyte)
	detail := fmt.Sprintf(
		"memory limit too large for this judge: the submission needs %d MB but the per-judgment budget is %d MB. "+
			"Lower the problem's memory limit, raise JUDGE_MAX_JUDGMENT_MEM_MB, or set a per-language memory_limit_kb override.",
		needBytes/megabyte, wp.maxJudgmentBytes/megabyte)
	wp.finish(ctx, submissionID, claimToken, model.StatusSE, 0, 0, 0, detail, nil)
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

// submissionJudgeStore is the claim protocol this pool runs against: read the
// row, fence it with a claim, write the result back only while the claim holds.
//
// Deliberately not store.SubmissionStore: a judge worker must not be able to
// list, void or rejudge submissions, and a test fake for it is three methods
// rather than thirty.
type submissionJudgeStore interface {
	GetByID(ctx context.Context, id string) (*model.Submission, error)
	ClaimPending(ctx context.Context, id, claimToken string) (bool, error)
	UpdateResultClaimed(ctx context.Context, id, claimToken string, status model.SubmissionStatus, score, timeUsed, memoryUsed int, compileOutput string, results []model.TestCaseResult) (bool, error)
}

// judgeProblemStore is what the worker reads and writes on a problem: the row it
// judges, and the solve counters it advances when a verdict lands.
type judgeProblemStore interface {
	GetByID(ctx context.Context, id string) (*model.Problem, error)
	UpdateCounts(ctx context.Context, id string, addSubmission, addAccepted int) error
}
