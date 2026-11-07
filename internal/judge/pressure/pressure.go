// Package pressure implements a lightweight resource-pressure gate for the
// judge worker pool.
//
// The problem it solves: when the server is under heavy load (e.g. a Java
// submission spawning a 400 MB JVM while Postgres is already swapping), starting
// another judgment immediately makes everything worse. The gate samples
// available RAM and the 1-minute CPU load average before every dispatch and
// holds the job in-hand, retrying on a short interval, until the server has
// enough headroom to absorb the new workload safely.
//
// Design choices:
//   - Reads /proc/meminfo and /proc/loadavg directly — no CGO, no external
//     library, works inside the Alpine/distroless backend image.
//   - Falls back to NoopSampler on non-Linux hosts (macOS dev, CI) so the
//     gate is a transparent no-op in those environments.
//   - ResourceSampler is an interface so tests can inject a fake.
//   - PressureGate.WaitUntilReady blocks until both thresholds are satisfied
//     or the context is cancelled. It logs a single warning on the first
//     overloaded sample and a single recovery message, avoiding log spam.
package pressure

import (
	"context"
	"log/slog"
	"runtime"
	"time"
)

// DefaultPollInterval is how often WaitUntilReady re-checks resources while
// the server is overloaded. 2 s is fast enough to react when a previous
// judgment finishes and slow enough not to hammer /proc.
const DefaultPollInterval = 2 * time.Second

// Snapshot holds a single resource sample.
type Snapshot struct {
	// FreeMemMB is the amount of RAM available to new processes, in MiB.
	// On Linux this is MemAvailable from /proc/meminfo, which accounts for
	// reclaimable page-cache and slab — a much more accurate "how much can I
	// allocate right now" than MemFree alone.
	FreeMemMB uint64

	// LoadAvg1 is the 1-minute load average reported by /proc/loadavg.
	// Dividing by runtime.NumCPU() gives per-core load; > 1.0 per core means
	// the run queue is longer than the cores can drain.
	LoadAvg1 float64
}

// ResourceSampler is the read interface the gate uses to observe the host.
// Inject a fake in tests; use NewLinuxSampler() or DefaultSampler() in
// production.
type ResourceSampler interface {
	Sample() (Snapshot, error)
}

// Thresholds carries the two knobs operators can tune.
type Thresholds struct {
	// MinFreeMemMB: refuse to start a new judgment when free RAM is below
	// this. A Java compilation can spike 400 MB, so 512 MB is a safe floor
	// for a server with concurrency ≥ 2.  0 disables the memory check.
	MinFreeMemMB uint64

	// MaxLoadPerCPU: refuse when (loadavg1 / numCPU) exceeds this ratio.
	// 1.5 means "allow up to 50% over-subscription on the run queue before
	// pausing new judgments". 0 disables the CPU check.
	MaxLoadPerCPU float64
}

// Disabled returns true when both thresholds are zero (gate is a no-op).
func (t Thresholds) Disabled() bool {
	return t.MinFreeMemMB == 0 && t.MaxLoadPerCPU == 0
}

// PressureGate wraps a ResourceSampler and two thresholds. The zero value is
// safe but does nothing (both thresholds zero → Disabled).
type PressureGate struct {
	sampler      ResourceSampler
	thresholds   Thresholds
	pollInterval time.Duration
	numCPU       int
}

// NewGate constructs a PressureGate.  sampler may be nil when thresholds are
// disabled (Disabled() == true), in which case WaitUntilReady returns
// immediately.
func NewGate(sampler ResourceSampler, t Thresholds) *PressureGate {
	return &PressureGate{
		sampler:      sampler,
		thresholds:   t,
		pollInterval: DefaultPollInterval,
		numCPU:       runtime.NumCPU(),
	}
}

// DefaultGate builds a gate using the platform-appropriate sampler and the
// given thresholds. On Linux this reads /proc; on other platforms it uses
// NoopSampler so the gate is transparent.
func DefaultGate(t Thresholds) *PressureGate {
	return NewGate(DefaultSampler(), t)
}

// IsOverloaded samples the host once and returns true when either threshold is
// exceeded. It never returns an error — a failed sample is treated as "not
// overloaded" (fail-open) so a /proc read error cannot permanently block
// judging.
func (g *PressureGate) IsOverloaded() bool {
	if g.thresholds.Disabled() || g.sampler == nil {
		return false
	}
	snap, err := g.sampler.Sample()
	if err != nil {
		slog.Warn("pressure: resource sample failed; treating as not overloaded", "error", err)
		return false
	}
	if g.thresholds.MinFreeMemMB > 0 && snap.FreeMemMB < g.thresholds.MinFreeMemMB {
		return true
	}
	if g.thresholds.MaxLoadPerCPU > 0 && g.numCPU > 0 {
		if snap.LoadAvg1/float64(g.numCPU) > g.thresholds.MaxLoadPerCPU {
			return true
		}
	}
	return false
}

// WaitUntilReady blocks until the server has enough headroom (IsOverloaded
// returns false) or the context is cancelled.
//
// It logs a single "pausing" message when overload is first detected and a
// single "resuming" message when it clears, to avoid flooding the log during a
// sustained high-load period.
func (g *PressureGate) WaitUntilReady(ctx context.Context) error {
	if !g.IsOverloaded() {
		return nil
	}
	slog.Warn("pressure: server resources are constrained; pausing judgment dispatch",
		"min_free_mem_mb", g.thresholds.MinFreeMemMB,
		"max_load_per_cpu", g.thresholds.MaxLoadPerCPU,
	)
	ticker := time.NewTicker(g.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if !g.IsOverloaded() {
				slog.Info("pressure: server resources recovered; resuming judgment dispatch")
				return nil
			}
		}
	}
}
