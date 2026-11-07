//go:build linux

package pressure

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LinuxSampler reads /proc/meminfo and /proc/loadavg to produce a Snapshot.
// Both files are always present on Linux kernels ≥ 2.6 and inside Docker
// containers (they reflect the host kernel's view, not the container's cgroup
// limit — which is exactly what we want, since go-judge runs in the host cgroup
// namespace).
type LinuxSampler struct{}

// NewLinuxSampler returns a ResourceSampler backed by /proc.
func NewLinuxSampler() ResourceSampler {
	return &LinuxSampler{}
}

// DefaultSampler returns the Linux /proc sampler on this platform.
func DefaultSampler() ResourceSampler {
	return NewLinuxSampler()
}

// Sample reads /proc/meminfo and /proc/loadavg and returns a Snapshot.
func (s *LinuxSampler) Sample() (Snapshot, error) {
	mem, err := readMemAvailableMB()
	if err != nil {
		return Snapshot{}, fmt.Errorf("pressure: read meminfo: %w", err)
	}
	load, err := readLoadAvg1()
	if err != nil {
		return Snapshot{}, fmt.Errorf("pressure: read loadavg: %w", err)
	}
	return Snapshot{FreeMemMB: mem, LoadAvg1: load}, nil
}

// readMemAvailableMB parses /proc/meminfo and returns MemAvailable in MiB.
//
// MemAvailable (kernel ≥ 3.14) is the best estimate of how much memory is
// available for starting new applications without swapping. It includes free
// memory plus memory that the kernel can reclaim from page-cache and slab.
// MemFree alone would severely under-count on a system that is using disk cache
// heavily (which Linux always does).
func readMemAvailableMB() (uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "MemAvailable:") {
			continue
		}
		// Format: "MemAvailable:   12345678 kB"
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("unexpected MemAvailable line: %q", line)
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse MemAvailable: %w", err)
		}
		return kb / 1024, nil // kB → MiB
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("MemAvailable not found in /proc/meminfo")
}

// readLoadAvg1 parses /proc/loadavg and returns the 1-minute load average.
//
// /proc/loadavg format: "0.52 0.58 0.59 1/347 12345"
// The first field is the 1-minute load average.
func readLoadAvg1() (float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("unexpected /proc/loadavg content: %q", string(data))
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("parse loadavg: %w", err)
	}
	return v, nil
}
