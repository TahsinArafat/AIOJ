package judge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

/**
 * The probe must fire on stalled work and only on stalled work.
 *
 * Two ways to get this wrong, and both are worse than having no probe at all:
 *
 *   - Too eager: an idle worker (no jobs) or a worker mid-judgment is declared
 *     dead, Docker restarts it, and a brief stall becomes a restart loop
 *     mid-contest. The judgments in flight are abandoned each time.
 *   - Too lax: a genuinely wedged worker reports healthy and nothing recovers,
 *     which is the bug this whole change exists to fix.
 *
 * So the cases below pin both edges, not just the middle.
 */
func TestHealthStateStalledOnlyForWorkPastTheDeadline(t *testing.T) {
	const timeout = 10 * time.Minute
	threshold := timeout + stallMargin

	t.Run("an idle worker is healthy", func(t *testing.T) {
		// The important negative: no jobs means nothing can be stuck. A
		// progress-based or liveness-based probe would be tempted to call this
		// unhealthy, which would restart a judge that simply had no work.
		h := NewHealthState()
		if h.IsStalled(timeout) {
			t.Fatal("an idle worker must be healthy: with no jobs in flight there is nothing that can be stuck")
		}
		if age := h.OldestJobAge(); age != 0 {
			t.Fatalf("OldestJobAge = %v with no jobs, want 0", age)
		}
	})

	t.Run("a fresh job is healthy", func(t *testing.T) {
		h := NewHealthState()
		h.BeginJob("s1")
		defer h.EndJob("s1")
		if h.IsStalled(timeout) {
			t.Fatal("a judgment that just started must not be called stalled")
		}
		if h.InFlight() != 1 {
			t.Fatalf("InFlight = %d, want 1", h.InFlight())
		}
	})

	t.Run("a job past the deadline is stalled", func(t *testing.T) {
		// Staged by backdating the recorded start, so this does not wait ten
		// minutes. The rule under test is the comparison, not the clock.
		h := NewHealthState()
		h.BeginJob("s1")
		defer h.EndJob("s1")
		h.mu.Lock()
		h.inFlight["s1"] = time.Now().Add(-(threshold + time.Second))
		h.mu.Unlock()

		if !h.IsStalled(timeout) {
			t.Fatalf("a judgment older than %v must be reported stalled, or nothing ever restarts a wedged worker", threshold)
		}
	})

	t.Run("a job just inside the threshold is healthy", func(t *testing.T) {
		// The boundary that keeps a slow-but-working judgment from triggering a
		// restart. Anything at or under the deadline plus margin must pass.
		h := NewHealthState()
		h.BeginJob("s1")
		defer h.EndJob("s1")
		h.mu.Lock()
		h.inFlight["s1"] = time.Now().Add(-(threshold - time.Second))
		h.mu.Unlock()

		if h.IsStalled(timeout) {
			t.Fatal("a judgment still within the deadline plus margin must not be called stalled")
		}
	})

	t.Run("the oldest job decides, not the newest", func(t *testing.T) {
		// Two slots, one wedged. Reporting on the newest would hide the wedge
		// forever, because a second job starting resets the newest timestamp.
		h := NewHealthState()
		h.BeginJob("old")
		h.BeginJob("new")
		defer h.EndJob("old")
		defer h.EndJob("new")
		h.mu.Lock()
		h.inFlight["old"] = time.Now().Add(-(threshold + time.Minute))
		h.mu.Unlock()

		if !h.IsStalled(timeout) {
			t.Fatal("a wedged slot must be detected even when another job started recently")
		}
	})

	t.Run("EndJob clears staleness", func(t *testing.T) {
		h := NewHealthState()
		h.BeginJob("s1")
		h.mu.Lock()
		h.inFlight["s1"] = time.Now().Add(-(threshold + time.Minute))
		h.mu.Unlock()
		if !h.IsStalled(timeout) {
			t.Fatal("precondition: expected the job to be stalled before it finished")
		}
		h.EndJob("s1")
		if h.IsStalled(timeout) {
			t.Fatal("a finished judgment must stop counting toward staleness")
		}
		if h.InFlight() != 0 {
			t.Fatalf("InFlight = %d after the only job ended, want 0", h.InFlight())
		}
	})
}

// The endpoint is what Docker actually calls, and its status code is the whole
// contract: 200 keeps the container, 503 restarts it. Driving the HTTP handler
// rather than HealthState directly is deliberate — a correct predicate behind a
// broken handler would still leave the judge un-recovered, which is the same
// "policy vs gate" trap as the scoreboard access bug.
func TestHealthEndpointStatusFollowsStallState(t *testing.T) {
	cases := []struct {
		name       string
		stage      func(h *HealthState, timeout time.Duration)
		wantStatus int
	}{
		{
			name:       "idle is healthy",
			stage:      func(*HealthState, time.Duration) {},
			wantStatus: http.StatusOK,
		},
		{
			name: "a judgment past the deadline is unhealthy",
			stage: func(h *HealthState, timeout time.Duration) {
				h.BeginJob("s1")
				h.mu.Lock()
				h.inFlight["s1"] = time.Now().Add(-(timeout + stallMargin + time.Minute))
				h.mu.Unlock()
			},
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const timeout = 10 * time.Minute
			h := NewHealthState()
			tc.stage(h, timeout)
			pool := &WorkerPool{health: h, judgmentTimeout: timeout}

			// ServeHealth installs the handler on a real listener; the same closure
			// is what the probe hits. Building the mux by hand here would test a
			// copy of the logic rather than the handler that ships.
			mux := pool.healthMux(timeout)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

			if rec.Code != tc.wantStatus {
				t.Fatalf("GET /healthz = %d, want %d (body=%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("health body is not JSON: %v (%s)", err, rec.Body.String())
			}
			if _, ok := payload["oldest_job_sec"]; !ok {
				t.Fatalf("health body has no oldest_job_sec, so an operator cannot see why: %v", payload)
			}
		})
	}
}
