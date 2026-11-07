package harness

import (
	"context"
	"sync"
)

// MemoryTranscript keeps turns in memory. Used by the in-process worker (where
// the job row already holds the durable state) and by tests.
//
// It is safe for concurrent use because a single job is processed by one
// worker but the loop may be inspected from an admin request at the same time.
type MemoryTranscript struct {
	mu    sync.Mutex
	turns map[string][]Turn
}

// NewMemoryTranscript builds an empty in-memory transcript.
func NewMemoryTranscript() *MemoryTranscript {
	return &MemoryTranscript{turns: make(map[string][]Turn)}
}

func (m *MemoryTranscript) Append(_ context.Context, jobID string, turn Turn) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.turns[jobID] = append(m.turns[jobID], turn)
	return nil
}

func (m *MemoryTranscript) Load(_ context.Context, jobID string) ([]Turn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	src := m.turns[jobID]
	out := make([]Turn, len(src))
	copy(out, src)
	return out, nil
}

// Len reports how many turns are stored for a job, for tests and diagnostics.
func (m *MemoryTranscript) Len(jobID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.turns[jobID])
}
