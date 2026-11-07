package problemgen

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/harness"
	"github.com/tahsinarafat/aioj/internal/model"
)

// JobStore is the slice of the generation-job store the worker needs.
//
// Declared here rather than importing the Postgres store so the worker can be
// tested against a fake, and so the separate-container deployment can point it
// at the same store without a second abstraction.
type JobStore interface {
	ClaimNext(ctx context.Context, token string) (*model.AIGenerationJob, error)
	Heartbeat(ctx context.Context, id, token, stage string, progress int) error
	IsCancellationRequested(ctx context.Context, id string) (bool, error)
	MarkCancelled(ctx context.Context, id, token string) error
	RequeueStale(ctx context.Context, older time.Duration) ([]string, error)
	SetManifest(ctx context.Context, id, token string, manifest, report json.RawMessage, ready bool, turns int) error
	Fail(ctx context.Context, id, token, code, message string, validation json.RawMessage) error
}

// ModelResolver builds a model client for a job. A nil modelID means "use the
// deployment default", which is what the single-model case does.
type ModelResolver func(ctx context.Context, modelID *string) (harness.Model, error)

// Worker drains generation jobs through the pipeline.
//
// It is the deployment seam: the same worker runs in-process under
// --mode=ai-worker or in its own container under --mode=ai-harness. Only the
// Runner and ModelResolver differ between those two, and both are injected.
type Worker struct {
	Jobs       JobStore
	Resolve    ModelResolver
	Runner     Runner
	Corpus     Corpus
	Transcript harness.Transcript
	Staging    string
	MaxBytes   int64
	Budget     harness.Budget

	// Poll is how often to look for work; StaleAfter is how long a job may go
	// without a heartbeat before it is requeued.
	Poll       time.Duration
	StaleAfter time.Duration
}

// Run drains jobs until the context is cancelled.
func (w *Worker) Run(ctx context.Context) {
	if w.Poll <= 0 {
		w.Poll = time.Second
	}
	if w.StaleAfter <= 0 {
		w.StaleAfter = 10 * time.Minute
	}
	ticker := time.NewTicker(w.Poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// A worker that died mid-run leaves the job "running" forever;
			// requeueing on a stale heartbeat is what makes the queue
			// self-healing.
			if _, err := w.Jobs.RequeueStale(ctx, w.StaleAfter); err != nil {
				slog.Warn("ai harness: requeue stale failed", "error", err)
			}
			w.processOne(ctx)
		}
	}
}

func (w *Worker) processOne(ctx context.Context) {
	token := uuid.NewString()
	job, err := w.Jobs.ClaimNext(ctx, token)
	if err != nil || job == nil {
		return
	}

	if cancelled, _ := w.Jobs.IsCancellationRequested(ctx, job.ID); cancelled {
		_ = w.Jobs.MarkCancelled(ctx, job.ID, token)
		return
	}

	brief, err := ParseBrief(job.NormalizedBrief)
	if err != nil {
		_ = w.Jobs.Fail(ctx, job.ID, token, "invalid_brief", err.Error(), nil)
		return
	}

	model, err := w.Resolve(ctx, job.AIModelID)
	if err != nil || model == nil {
		_ = w.Jobs.Fail(ctx, job.ID, token, "connection_unavailable", "AI connection is unavailable", nil)
		return
	}

	_ = w.Jobs.Heartbeat(ctx, job.ID, token, "drafting", 10)

	pipeline := &Pipeline{
		Model:       model,
		Runner:      w.Runner,
		Corpus:      w.Corpus,
		Transcript:  w.Transcript,
		Budget:      w.Budget,
		StagingRoot: w.Staging,
		MaxBytes:    w.MaxBytes,
	}

	manifest, err := pipeline.Run(ctx, job.ID, brief.Render())
	if err != nil {
		_ = w.Jobs.Fail(ctx, job.ID, token, "pipeline_error", err.Error(), nil)
		return
	}

	if cancelled, _ := w.Jobs.IsCancellationRequested(ctx, job.ID); cancelled {
		_ = w.Jobs.MarkCancelled(ctx, job.ID, token)
		return
	}

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		_ = w.Jobs.Fail(ctx, job.ID, token, "manifest_encode", err.Error(), nil)
		return
	}
	reportJSON, _ := json.Marshal(manifest.Report)

	if err := w.Jobs.SetManifest(ctx, job.ID, token, manifestJSON, reportJSON, manifest.Report.Ready, manifest.Usage.Materialized); err != nil {
		slog.Error("ai harness: persist manifest failed", "job", job.ID, "error", err)
		return
	}
	slog.Info("ai harness: job finished",
		"job", job.ID, "ready", manifest.Report.Ready,
		"cases", len(manifest.Cases), "blocked", manifest.Report.Blocked)
}

// DefaultBudget derives the loop budget from config.
func DefaultBudget(maxTurns int) harness.Budget {
	b := harness.DefaultBudget()
	if maxTurns > 0 {
		b.MaxTurns = maxTurns
	}
	return b
}

// validate guards against a worker that would silently do nothing.
func (w *Worker) validate() error {
	if w.Jobs == nil {
		return fmt.Errorf("problemgen: a job store is required")
	}
	if w.Resolve == nil {
		return fmt.Errorf("problemgen: a model resolver is required")
	}
	if w.Runner == nil {
		return fmt.Errorf("problemgen: a runner is required")
	}
	return nil
}

// Ready reports whether the worker is safe to start.
func (w *Worker) Ready() error { return w.validate() }
