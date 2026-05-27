package queue

import (
	"context"
	"sync"
)

type MemoryQueue struct {
	mu    sync.Mutex
	cond  *sync.Cond
	queue []string
	done  bool
}

func NewMemory() *MemoryQueue {
	m := &MemoryQueue{}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *MemoryQueue) Enqueue(_ context.Context, id string) error {
	m.mu.Lock()
	m.queue = append(m.queue, id)
	m.cond.Signal()
	m.mu.Unlock()
	return nil
}

func (m *MemoryQueue) Dequeue(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for len(m.queue) == 0 && !m.done {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		m.cond.Wait()
	}
	if m.done && len(m.queue) == 0 {
		return "", context.Canceled
	}
	id := m.queue[0]
	m.queue = m.queue[1:]
	return id, nil
}

func (m *MemoryQueue) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue)
}

func (m *MemoryQueue) Close() error {
	m.mu.Lock()
	m.done = true
	m.cond.Broadcast()
	m.mu.Unlock()
	return nil
}
