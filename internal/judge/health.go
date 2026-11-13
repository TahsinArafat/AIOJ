package judge

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

/**
 * A judge-worker that has stopped judging has to be able to say so.
 *
 * This exists because of a real failure: the judge and judge-worker containers
 * occasionally stopped compiling, running test cases and serving the IDE, and
 * restarting the two containers fixed it. Docker never restarted them on its own
 * and could not have — `restart: unless-stopped` only acts on a container that
 * has *exited*, and these were still running, merely parked. A process blocked in
 * a legitimate wait looks exactly like a healthy one unless it is asked a question
 * that a healthy process answers quickly and a stuck one cannot.
 *
 * So the worker serves one endpoint. It reports the age of the oldest judgment
 * still in flight and returns 503 once that exceeds the threshold, which makes
 * Docker's healthcheck declare the container unhealthy and restart it. That is
 * the "self recovery" an operator was doing by hand.
 *
 * The signal is *stuck work*, not liveness and not loop progress:
 *
 *   - Liveness ("is the process up") is worthless here — the whole problem is a
 *     process that is up.
 *   - Loop progress is nearly as bad: an idle worker blocked in Dequeue has made
 *     no progress and is perfectly healthy, so a progress-based probe would
 *     restart a judge that simply had nothing to do.
 *   - A judgment that has outlived its own deadline is unambiguous. The worker
 *     enforces that deadline itself, so a job older than the deadline plus a
 *     margin means the enforcement path is not running either — the loop is
 *     wedged, and only a restart clears it.
 */

// stallMargin is added to the judgment deadline to decide "wedged". It must be
// comfortably positive: too small and one slow judgment triggers a restart loop,
// which would turn a brief stall into permanent churn mid-contest.
const stallMargin = 5 * time.Minute

// HealthState tracks in-flight judgments so the probe can answer "is any single
// judgment older than it could possibly be".
//
// Guarded by a mutex rather than atomics because the answer needs the *oldest*
// start time across a handful of entries, and the pool's concurrency is 2–4 --
// contention is irrelevant, correctness is not.
type HealthState struct {
	mu       sync.Mutex
	inFlight map[string]time.Time
	started  time.Time
}

func NewHealthState() *HealthState {
	return &HealthState{inFlight: make(map[string]time.Time), started: time.Now()}
}

func (h *HealthState) BeginJob(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.inFlight[id] = time.Now()
}

func (h *HealthState) EndJob(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.inFlight, id)
}

// OldestJobAge returns the age of the longest-running judgment, or 0 when the
// worker holds none. Zero means healthy by construction: an idle judge cannot be
// stuck.
func (h *HealthState) OldestJobAge() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	var oldest time.Duration
	for _, startedAt := range h.inFlight {
		if age := time.Since(startedAt); age > oldest {
			oldest = age
		}
	}
	return oldest
}

func (h *HealthState) InFlight() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.inFlight)
}

// IsStalled reports whether any judgment has outlived the deadline that should
// have failed it. judgmentTimeout is the same value the worker enforces, so this
// is self-consistent: the probe fires only when enforcement demonstrably failed.
func (h *HealthState) IsStalled(judgmentTimeout time.Duration) bool {
	return h.OldestJobAge() > judgmentTimeout+stallMargin
}

// effectiveJudgmentTimeout mirrors the fallback the worker applies before arming
// the deadline. The probe must compare against the same value the worker
// enforces, or a zero-valued field would make the probe believe every judgment is
// already overdue.
func (wp *WorkerPool) effectiveJudgmentTimeout() time.Duration {
	if wp.judgmentTimeout > 0 {
		return wp.judgmentTimeout
	}
	return DefaultJudgmentTimeout
}

// healthMux builds the probe handler. Extracted from ServeHealth so a test can
// drive the exact handler that ships instead of a reimplementation of it — the
// status code IS the contract Docker acts on, so verifying a copy would prove
// nothing.
func (wp *WorkerPool) healthMux(timeout time.Duration) http.Handler {
	threshold := timeout + stallMargin
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		age := wp.health.OldestJobAge()
		stalled := wp.health.IsStalled(timeout)
		body := map[string]any{
			"status":           map[bool]string{true: "stalled", false: "ok"}[stalled],
			"in_flight":        wp.health.InFlight(),
			"oldest_job_sec":   age.Seconds(),
			"stall_after_sec":  threshold.Seconds(),
			"judgment_timeout": timeout.String(),
			"uptime_sec":       time.Since(wp.health.started).Seconds(),
		}
		w.Header().Set("Content-Type", "application/json")
		if stalled {
			slog.Error("health probe reports a stalled judge worker: a judgment has outlived the deadline that should have failed it",
				"oldest_job", age.String(), "threshold", threshold.String(),
				"in_flight", wp.health.InFlight())
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(body)
	})
	return mux
}

// ServeHealth starts an HTTP listener that reports worker health and blocks until
// ctx is done. The caller runs it in a goroutine.
//
// A failure to listen is logged rather than fatal: an operator who forgot to
// publish the port should get a judge that still works, not one that refuses to
// start. Docker reports the container unhealthy in that case, which is visible
// and fixable rather than a crash loop.
func (wp *WorkerPool) ServeHealth(ctx context.Context, addr string) {
	if wp.health == nil || addr == "" {
		return
	}
	timeout := wp.effectiveJudgmentTimeout()
	mux := wp.healthMux(timeout)

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	slog.Info("judge-worker health endpoint listening", "addr", addr, "stall_after", (timeout + stallMargin).String())
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("judge-worker health endpoint stopped", "addr", addr, "error", err)
	}
}
