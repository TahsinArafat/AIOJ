package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tahsinarafat/aioj/internal/ai"
	"github.com/tahsinarafat/aioj/internal/config"
	"github.com/tahsinarafat/aioj/internal/harness"
	"github.com/tahsinarafat/aioj/internal/harness/problemgen"
	"github.com/tahsinarafat/aioj/internal/judge"
	"github.com/tahsinarafat/aioj/internal/judge/executor"
	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/store"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

/**
 * The non-server run modes.
 *
 * main() had grown to 919 lines and did four unrelated jobs: the shared wiring
 * every mode needs, and then one inline block per mode. Each block ended with the
 * same signal-wait-shutdown sequence, so reading any one of them meant scrolling
 * past the other three.
 *
 * Each mode is now a function here, taking exactly what it uses. main() keeps the
 * wiring and dispatches.
 */

// awaitShutdown blocks until SIGINT or SIGTERM, then runs the mode's own
// teardown.
//
// The signal handling was copied into all three modes and differed in what it
// did afterwards, which is the part that matters: a judge worker must drain, or
// cancelling alone abandons in-flight judgments mid-write and the old code
// recorded a System Error over them. So the wait is shared and the teardown is
// the caller's.
func awaitShutdown(cancel context.CancelFunc, teardown func()) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
	if teardown != nil {
		teardown()
	}
}

// runJudgeWorker runs a dedicated judge process.
//
// Only these processes consume the shared queue; the API server must not silently
// become a third decider.
func runJudgeWorker(
	ctx context.Context,
	cancel context.CancelFunc,
	cfg *config.Config,
	submissionStore store.SubmissionStore,
	judgeQueue queue.JudgeQueue,
	workerPool *judge.WorkerPool,
) {
	reclaimer := judge.NewReclaimer(submissionStore, judgeQueue, judge.DefaultStaleClaimAfter, judge.DefaultReclaimInterval)
	go reclaimer.Start(ctx)
	go workerPool.Start(ctx)

	// The probe Docker's healthcheck calls. This process serves no HTTP
	// otherwise, so before this a wedged worker was undetectable from outside:
	// the container stayed "running" and `restart: unless-stopped` never fired,
	// which is why a human had to restart it by hand.
	if cfg.Judge.HealthAddr != "" {
		go workerPool.ServeHealth(ctx, cfg.Judge.HealthAddr)
	}

	slog.Info("running in judge-worker mode, waiting for submissions")
	awaitShutdown(cancel, func() {
		slog.Info("judge-worker shutting down")
		// Drain before returning: cancelling alone abandons in-flight judgments
		// mid-write.
		workerPool.Stop()
		judgeQueue.Close()
	})
}

// runAIWorker runs the single-shot AI generation worker and its retention sweep.
func runAIWorker(
	cfg *config.Config,
	cancel context.CancelFunc,
	startWorkers func(),
	startRetention func(),
) {
	if !cfg.AI.Enabled {
		log.Fatal("ai-worker mode requires AI_ENABLED=true")
	}
	startWorkers()
	startRetention()
	slog.Info("running in ai-worker mode, waiting for generation jobs")
	awaitShutdown(cancel, nil)
}

// runAIHarness runs the multi-turn generator pipeline: the model calls tools, and
// the tools materialize test data by executing generated code inside go-judge.
//
// A separate mode (and, in the shipped compose file, a separate container) so a
// long, token-heavy run cannot compete with API traffic.
func runAIHarness(
	ctx context.Context,
	cancel context.CancelFunc,
	cfg *config.Config,
	execClient *executor.Client,
	db *sql.DB,
	aiGenerationStore store.AIGenerationStore,
	aiModelStore store.AIModelStore,
	problemStore store.ProblemStore,
) {
	if !cfg.AI.Enabled || !cfg.AI.Harness {
		log.Fatal("ai-harness mode requires AI_ENABLED=true and AI_HARNESS_ENABLED=true")
	}

	problemgen.MaxN = cfg.AI.MaxCaseN
	runner, err := problemgen.NewJudgeRunner(execClient, cfg.LangDir)
	if err != nil {
		log.Fatalf("ai harness: %v", err)
	}

	harnessWorker := &problemgen.Worker{
		Jobs:       aiGenerationStore,
		Runner:     runner,
		Corpus:     postgres.NewNoveltyCorpus(db),
		Problems:   problemStore,
		Transcript: postgres.NewHarnessTranscript(db),
		Staging:    cfg.AI.StagedTestdataRoot,
		MaxBytes:   cfg.AI.MaxJobBytes,
		Budget:     problemgen.DefaultBudget(cfg.AI.RepairTurns),
		Resolve: func(ctx context.Context, modelID *string) (harness.Model, error) {
			m, err := aiModelStore.GetDefault(ctx)
			if modelID != nil {
				m, err = aiModelStore.GetByID(ctx, *modelID)
			}
			if err != nil || m == nil {
				return nil, fmt.Errorf("no AI model is enabled")
			}
			return ai.NewWithOptions(m.Endpoint, m.APIKey, m.ModelName, ai.ClientOptions{
				Mode:                m.StructuredOutputMode,
				Timeout:             time.Duration(m.RequestTimeoutSeconds) * time.Second,
				MaxOutputTokens:     m.MaxOutputTokens,
				AllowPrivateNetwork: m.AllowPrivateNetwork,
			}), nil
		},
	}
	if err := harnessWorker.Ready(); err != nil {
		log.Fatalf("ai harness: %v", err)
	}

	go harnessWorker.Run(ctx)
	slog.Info("running in ai-harness mode, waiting for generation jobs")
	awaitShutdown(cancel, nil)
}
