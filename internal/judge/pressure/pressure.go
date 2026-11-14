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
//   - PressureGate.Admit is the memory-aware variant: it takes the size of the
//     judgment being dispatched, because a flat free-memory floor cannot tell a
//     50 MB C++ run from a 4 GB Java run.
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

// megabyte converts the sampler's MiB reading into bytes for comparison against
// a submission's memory requirement.
const megabyte uint64 = 1 << 20

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

	// MemHeadroomMB is slack required on top of a judgment's own memory
	// requirement before it is admitted. The requirement is what the sandbox may
	// allocate; the compile step that precedes it, the judge process itself and
	// the rest of the stack need room that the requirement does not describe.
	// 0 means the requirement alone is the bar.
	MemHeadroomMB uint64
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

// Admit blocks until the host can absorb a judgment needing needBytes of RAM on
// top of reservedBytes already committed to in-flight judgments, and the CPU
// load is within bounds. It returns ctx.Err() when the context is cancelled
// while waiting.
//
// Why this exists alongside WaitUntilReady: that method only knows a flat
// free-memory floor, so on a host with 600 MB free it admits a judgment that is
// about to ask the sandbox for 2 GB (4 GB for Java, whose MemoryLimitMultiplier
// is 2.0). The sandbox then fails to allocate or the host starts swapping, and
// every other container on the machine pays for it. Passing the requirement in
// makes the wait proportional to what is actually coming.
//
// reservedBytes covers the window between "admitted" and "the sandbox has
// actually allocated": MemAvailable still reports those bytes as free for the
// whole compile phase, so without subtracting them two large judgments are
// admitted back to back on the strength of one pool of memory.
//
// A needBytes of 0 means "requirement unknown" and falls back to the flat floor.
func (g *PressureGate) Admit(ctx context.Context, needBytes, reservedBytes uint64) error {
	if g.thresholds.Disabled() || g.sampler == nil {
		return nil
	}
	if !g.admissionBlocked(needBytes, reservedBytes) {
		return nil
	}
	slog.Warn("pressure: waiting for resources to absorb this judgment",
		"need_mb", needBytes/megabyte,
		"reserved_mb", reservedBytes/megabyte,
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
			if !g.admissionBlocked(needBytes, reservedBytes) {
				slog.Info("pressure: resources recovered; dispatching judgment",
					"need_mb", needBytes/megabyte)
				return nil
			}
		}
	}
}

// admissionBlocked samples once and reports whether this judgment must wait.
//
// It combines the requirement-aware memory check with the same CPU check
// IsOverloaded applies, so a single gate answers both dimensions.
func (g *PressureGate) admissionBlocked(needBytes, reservedBytes uint64) bool {
	snap, err := g.sampler.Sample()
	if err != nil {
		slog.Warn("pressure: resource sample failed; treating as not overloaded", "error", err)
		return false
	}
	if g.thresholds.MinFreeMemMB > 0 && snap.FreeMemMB < g.thresholds.MinFreeMemMB {
		return true
	}
	free := snap.FreeMemMB * megabyte
	if reservedBytes >= free {
		free = 0
	} else {
		free -= reservedBytes
	}
	// Headroom is added to the requirement, not subtracted from free memory, so
	// the log line above can still report the judgment's real size.
	required := needBytes + g.thresholds.MemHeadroomMB*megabyte
	if required > free {
		return true
	}
	if g.thresholds.MaxLoadPerCPU > 0 && g.numCPU > 0 {
		if snap.LoadAvg1/float64(g.numCPU) > g.thresholds.MaxLoadPerCPU {
			return true
		}
	}
	return false
}
