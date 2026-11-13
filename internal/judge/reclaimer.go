package judge

import (
	"context"
	"log/slog"
	"time"

	"github.com/tahsinarafat/aioj/internal/queue"
	"github.com/tahsinarafat/aioj/internal/store"
)

// Default reclaim tuning.
//
// staleClaimAfter must comfortably exceed the slowest legitimate judgment
// (cold compile + worst-case problem) or a live judgment gets stolen and
// re-judged. reclaimInterval is how often we sweep.
const (
	DefaultStaleClaimAfter = 15 * time.Minute
	DefaultReclaimInterval = 2 * time.Minute
)

// Reclaimer re-enqueues submissions that were abandoned mid-judgment.
//
// Failure mode it fixes: a judge-worker sets status='judging', then the
// process is killed (deploy, OOM, crash) before UpdateResult runs. The queue
// entry was already popped, so nothing will ever pick the row up again and the
// submission is stuck on "Judging..." forever. On startup and on a ticker, this
// sweeps those orphans back to 'pending' and re-enqueues them.
//
// SubmissionStore is deliberately the public persistence seam: RequeueStale
// resets a row and ListPending reconciles the database with the queue. The
// latter is what makes a failed Redis enqueue recoverable on the next sweep.
type Reclaimer struct {
	subStore staleReclaimer
	queue    queue.JudgeQueue
	// staleAfter is how long a row may sit in 'judging' before it is presumed
	// abandoned. Interval is the sweep period.
	staleAfter time.Duration
	interval   time.Duration
}

// staleReclaimer is the pair of operations this sweep needs. Narrower than
// store.SubmissionStore, and narrower than store.SubmissionJudgeQueue: the
// reclaimer never claims a row or writes a result, it only puts abandoned rows
// back on the queue.
type staleReclaimer interface {
	RequeueStale(ctx context.Context, olderThan time.Duration) ([]string, error)
	ListPendingWithContext(ctx context.Context, limit int) ([]store.PendingSubmissionInfo, error)
}

// NewReclaimer builds a Reclaimer. Non-positive values fall back to the
// package defaults so a zero-valued config cannot accidentally steal live
// judgments (staleAfter=0 would reclaim everything immediately).
func NewReclaimer(subStore staleReclaimer, q queue.JudgeQueue, staleAfter, interval time.Duration) *Reclaimer {
	if staleAfter <= 0 {
		staleAfter = DefaultStaleClaimAfter
	}
	if interval <= 0 {
		interval = DefaultReclaimInterval
	}
	return &Reclaimer{subStore: subStore, queue: q, staleAfter: staleAfter, interval: interval}
}

// Start runs one immediate sweep (so a restart recovers orphans at once, not
// after the first tick) and then sweeps on the ticker until ctx is done.
func (r *Reclaimer) Start(ctx context.Context) {
	r.Sweep(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.Sweep(ctx)
		}
	}
}

// Sweep performs exactly one reclaim pass and returns the number of queue
// entries successfully ensured. Errors are logged, never fatal: a failed
// sweep must not take the worker down.
func (r *Reclaimer) Sweep(ctx context.Context) int {
	if _, err := r.subStore.RequeueStale(ctx, r.staleAfter); err != nil {
		slog.Error("stale-claim sweep failed", "error", err)
		return 0
	}

	// The database is the durable source of truth. Reconcile pending rows on
	// every sweep: if Redis was unavailable after a row was created or reset,
	// the next pass re-enqueues it. Submission handlers enqueue the common
	// path; this is the lossless fallback. RequeueStale has already returned
	// the reclaimed IDs, but they are pending now and are included in this
	// single authoritative list, avoiding duplicate queue entries.
	//
	// ListPendingWithContext returns each submission's contest window so we
	// can re-enqueue live-contest submissions at PriorityContest (1) instead
	// of the flat priority=1 that previously treated every reclaimed submission
	// the same as background practice work.
	infos, err := r.subStore.ListPendingWithContext(ctx, 1000)
	if err != nil {
		slog.Error("pending-submission reconciliation failed", "error", err)
		return 0
	}

	now := time.Now()
	queued := 0
	for _, info := range infos {
		// Assign priority from the submission's own record, not just the
		// contest clock. An upsolve submitted after the window must not be
		// promoted to contest priority even if the contest is somehow still
		// marked live (a stretched window, a clock skew) — that would let
		// practice traffic overtake real contestants.
		priority := 2 // PriorityNormal — matches the handler constants
		if !info.IsUpsolving && info.ContestStart != nil && info.ContestEnd != nil &&
			now.After(*info.ContestStart) && now.Before(*info.ContestEnd) {
			priority = 1 // PriorityContest
		}
		if err := r.queue.Enqueue(ctx, info.ID, priority); err != nil {
			slog.Error("re-enqueue of pending submission failed", "id", info.ID, "error", err)
			continue
		}
		queued++
	}
	if queued > 0 {
		slog.Info("reconciled pending submissions", "count", queued, "stale_after", r.staleAfter.String())
	}
	return queued
}
