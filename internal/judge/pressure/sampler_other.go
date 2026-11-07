//go:build !linux

package pressure

// DefaultSampler returns a NoopSampler on non-Linux platforms (macOS, Windows).
// The PressureGate is therefore always "not overloaded" in development and CI,
// so it has zero effect on local runs or the GitHub Actions environment.
func DefaultSampler() ResourceSampler {
	return NoopSampler{}
}

// NoopSampler is a ResourceSampler that always reports infinite free memory
// and zero load. Use it in tests or on non-Linux hosts.
type NoopSampler struct{}

func (NoopSampler) Sample() (Snapshot, error) {
	return Snapshot{FreeMemMB: ^uint64(0), LoadAvg1: 0}, nil
}
