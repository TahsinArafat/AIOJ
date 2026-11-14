package pressure_test

import (
	"context"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/judge/pressure"
)

// fakeSampler is a controllable ResourceSampler for tests.
type fakeSampler struct {
	snap pressure.Snapshot
	err  error
}

func (f *fakeSampler) Sample() (pressure.Snapshot, error) {
	return f.snap, f.err
}

func TestIsOverloaded_BothDisabled(t *testing.T) {
	gate := pressure.NewGate(&fakeSampler{snap: pressure.Snapshot{FreeMemMB: 0, LoadAvg1: 999}},
		pressure.Thresholds{MinFreeMemMB: 0, MaxLoadPerCPU: 0})
	if gate.IsOverloaded() {
		t.Fatal("gate with both thresholds=0 must never be overloaded")
	}
}

func TestIsOverloaded_MemoryThreshold(t *testing.T) {
	cases := []struct {
		freeMemMB uint64
		threshold uint64
		want      bool
	}{
		{freeMemMB: 1024, threshold: 512, want: false}, // plenty of RAM
		{freeMemMB: 512, threshold: 512, want: false},  // exactly at threshold — OK
		{freeMemMB: 511, threshold: 512, want: true},   // one MiB below — overloaded
		{freeMemMB: 0, threshold: 512, want: true},     // empty — overloaded
	}
	for _, tc := range cases {
		gate := pressure.NewGate(
			&fakeSampler{snap: pressure.Snapshot{FreeMemMB: tc.freeMemMB, LoadAvg1: 0}},
			pressure.Thresholds{MinFreeMemMB: tc.threshold, MaxLoadPerCPU: 0},
		)
		if got := gate.IsOverloaded(); got != tc.want {
			t.Errorf("freeMemMB=%d threshold=%d: IsOverloaded()=%v want %v",
				tc.freeMemMB, tc.threshold, got, tc.want)
		}
	}
}

func TestIsOverloaded_CPUThreshold(t *testing.T) {
	// Tests run on a machine with ≥1 CPU. We override numCPU indirectly
	// through the load values: on any machine, loadAvg1=0 → not overloaded;
	// loadAvg1=1000 / numCPU will far exceed any threshold.
	t.Run("not_overloaded", func(t *testing.T) {
		gate := pressure.NewGate(
			&fakeSampler{snap: pressure.Snapshot{FreeMemMB: ^uint64(0), LoadAvg1: 0}},
			pressure.Thresholds{MinFreeMemMB: 0, MaxLoadPerCPU: 1.5},
		)
		if gate.IsOverloaded() {
			t.Fatal("load=0 must not be overloaded")
		}
	})
	t.Run("overloaded", func(t *testing.T) {
		gate := pressure.NewGate(
			// 10000 / numCPU will always exceed 1.5 on any realistic machine.
			&fakeSampler{snap: pressure.Snapshot{FreeMemMB: ^uint64(0), LoadAvg1: 10000}},
			pressure.Thresholds{MinFreeMemMB: 0, MaxLoadPerCPU: 1.5},
		)
		if !gate.IsOverloaded() {
			t.Fatal("load=10000 must always be overloaded with threshold=1.5")
		}
	})
}

func TestIsOverloaded_FailOpen(t *testing.T) {
	// A failing sampler must not block judging — gate should report not-overloaded.
	gate := pressure.NewGate(
		&fakeSampler{err: context.DeadlineExceeded},
		pressure.Thresholds{MinFreeMemMB: 512, MaxLoadPerCPU: 1.5},
	)
	if gate.IsOverloaded() {
		t.Fatal("sampler error must fail-open (not overloaded)")
	}
}

func TestWaitUntilReady_ImmediateWhenNotOverloaded(t *testing.T) {
	gate := pressure.NewGate(
		&fakeSampler{snap: pressure.Snapshot{FreeMemMB: 2048, LoadAvg1: 0}},
		pressure.Thresholds{MinFreeMemMB: 512, MaxLoadPerCPU: 1.5},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := gate.WaitUntilReady(ctx); err != nil {
		t.Fatalf("WaitUntilReady returned error when not overloaded: %v", err)
	}
}

func TestWaitUntilReady_CancelledWhileWaiting(t *testing.T) {
	// Sampler always says overloaded — gate should block until ctx is cancelled.
	gate := pressure.NewGate(
		&fakeSampler{snap: pressure.Snapshot{FreeMemMB: 0, LoadAvg1: 0}},
		pressure.Thresholds{MinFreeMemMB: 512},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := gate.WaitUntilReady(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	// The gate must have waited (not returned instantly).
	if elapsed < 100*time.Millisecond {
		t.Fatalf("gate returned too quickly (%v); expected it to block", elapsed)
	}
}

func TestWaitUntilReady_ClearsAfterOnePoll(t *testing.T) {
	// First sample: overloaded. Second: OK. The gate's poll interval is 2 s by
	// default, so we shorten the test timeout to catch a logic error without
	// waiting 2 s. We can't set pollInterval from outside, so we set a 3 s
	// context deadline — the gate should return within that after one poll tick.
	callCount := 0
	sampler := &callCountSampler{
		responses: []pressure.Snapshot{
			{FreeMemMB: 0},    // first call: overloaded
			{FreeMemMB: 2048}, // second call: OK
			{FreeMemMB: 2048}, // extra calls: OK
		},
		callCount: &callCount,
	}
	gate := pressure.NewGate(sampler, pressure.Thresholds{MinFreeMemMB: 512})

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	if err := gate.WaitUntilReady(ctx); err != nil {
		t.Fatalf("WaitUntilReady returned error: %v", err)
	}
	// IsOverloaded() was called once (immediately) + at least once from the ticker.
	if callCount < 2 {
		t.Fatalf("expected ≥ 2 sample calls, got %d", callCount)
	}
}

// callCountSampler cycles through canned responses, counting calls.
type callCountSampler struct {
	responses []pressure.Snapshot
	callCount *int
	idx       int
}

func (c *callCountSampler) Sample() (pressure.Snapshot, error) {
	*c.callCount++
	if c.idx >= len(c.responses) {
		return c.responses[len(c.responses)-1], nil
	}
	s := c.responses[c.idx]
	c.idx++
	return s, nil
}

func TestAdmit_ImmediateWhenEnoughMemory(t *testing.T) {
	cases := []struct {
		name       string
		freeMB     uint64
		needBytes  uint64
		reserved   uint64
		headroomMB uint64
		wantBlock  bool
	}{
		{
			name: "ample memory", freeMB: 4096, needBytes: 512 << 20,
			wantBlock: false,
		},
		{
			// The exact bug this replaced: a flat 512 MB floor says "go" while
			// the judgment is about to ask for 2 GB.
			name: "big judgment into small free memory", freeMB: 600, needBytes: 2048 << 20,
			wantBlock: true,
		},
		{
			name: "exactly enough", freeMB: 2048, needBytes: 2048 << 20,
			wantBlock: false,
		},
		{
			name: "one MB short", freeMB: 2047, needBytes: 2048 << 20,
			wantBlock: true,
		},
		{
			// Reservations from in-flight judgments must be subtracted, or two
			// 2 GB judgments are admitted on one pool of memory.
			name: "reserved by in-flight judgment", freeMB: 2048, needBytes: 2048 << 20,
			reserved: 1536 << 20, wantBlock: true,
		},
		{
			// need=0 means "requirement unknown"; it must never block on the
			// memory dimension, even when reservations exceed free RAM.
			name: "unknown requirement never blocks on memory", freeMB: 1024, needBytes: 0,
			reserved: 4096 << 20, wantBlock: false,
		},
		{
			name: "headroom pushes over the line", freeMB: 2300, needBytes: 2048 << 20,
			headroomMB: 512, wantBlock: true,
		},
		{
			name: "headroom fits", freeMB: 4096, needBytes: 2048 << 20,
			headroomMB: 512, wantBlock: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gate := pressure.NewGate(
				&fakeSampler{snap: pressure.Snapshot{FreeMemMB: tc.freeMB}},
				pressure.Thresholds{MinFreeMemMB: 512, MemHeadroomMB: tc.headroomMB},
			)
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()

			start := time.Now()
			err := gate.Admit(ctx, tc.needBytes, tc.reserved)
			elapsed := time.Since(start)

			if tc.wantBlock {
				if err == nil {
					t.Fatal("expected Admit to block until context cancellation, got nil")
				}
				if elapsed < 100*time.Millisecond {
					t.Fatalf("Admit returned too quickly (%v); expected it to wait", elapsed)
				}
				return
			}
			if err != nil {
				t.Fatalf("Admit returned error when it should admit: %v", err)
			}
		})
	}
}

func TestAdmit_RespectsFlatFloorIndependently(t *testing.T) {
	// A tiny judgment must still wait when the server is below the floor: the
	// floor protects the whole stack, not just this submission.
	gate := pressure.NewGate(
		&fakeSampler{snap: pressure.Snapshot{FreeMemMB: 100}},
		pressure.Thresholds{MinFreeMemMB: 512},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := gate.Admit(ctx, 1<<20, 0); err == nil {
		t.Fatal("Admit must honor MinFreeMemMB even for a tiny judgment")
	}
}

func TestAdmit_CPUThresholdApplies(t *testing.T) {
	gate := pressure.NewGate(
		&fakeSampler{snap: pressure.Snapshot{FreeMemMB: ^uint64(0), LoadAvg1: 10000}},
		pressure.Thresholds{MaxLoadPerCPU: 1.5},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := gate.Admit(ctx, 1<<20, 0); err == nil {
		t.Fatal("Admit must block when the CPU load threshold is exceeded")
	}
}

func TestAdmit_DisabledGateIsTransparent(t *testing.T) {
	gate := pressure.NewGate(
		&fakeSampler{snap: pressure.Snapshot{FreeMemMB: 0, LoadAvg1: 999}},
		pressure.Thresholds{},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := gate.Admit(ctx, 4096<<20, 0); err != nil {
		t.Fatalf("disabled gate must admit immediately, got %v", err)
	}
}

func TestAdmit_FailOpenOnSamplerError(t *testing.T) {
	gate := pressure.NewGate(
		&fakeSampler{err: context.DeadlineExceeded},
		pressure.Thresholds{MinFreeMemMB: 512},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := gate.Admit(ctx, 4096<<20, 0); err != nil {
		t.Fatalf("sampler error must fail open, got %v", err)
	}
}

func TestAdmit_ResumesWhenMemoryFrees(t *testing.T) {
	callCount := 0
	sampler := &callCountSampler{
		responses: []pressure.Snapshot{
			{FreeMemMB: 256},  // too small for a 2 GB judgment
			{FreeMemMB: 4096}, // memory freed
			{FreeMemMB: 4096},
		},
		callCount: &callCount,
	}
	gate := pressure.NewGate(sampler, pressure.Thresholds{MinFreeMemMB: 128})
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := gate.Admit(ctx, 2048<<20, 0); err != nil {
		t.Fatalf("Admit should have resumed once memory freed: %v", err)
	}
	if callCount < 2 {
		t.Fatalf("expected ≥ 2 samples, got %d", callCount)
	}
}
