package queue

import (
	"container/heap"
	"context"
	"sync"
)

type queueItem struct {
	id       string
	priority int
	index    int
}

type priorityQueue []*queueItem

func (pq priorityQueue) Len() int { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool { return pq[i].priority < pq[j].priority }
func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}
func (pq *priorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*queueItem)
	item.index = n
	*pq = append(*pq, item)
}
func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

type MemoryQueue struct {
	mu   sync.Mutex
	cond *sync.Cond
	heap priorityQueue
	done bool
}

func NewMemory() *MemoryQueue {
	m := &MemoryQueue{heap: make(priorityQueue, 0)}
	m.cond = sync.NewCond(&m.mu)
	return m
}

func (m *MemoryQueue) Enqueue(_ context.Context, id string, priority int) error {
	m.mu.Lock()
	heap.Push(&m.heap, &queueItem{id: id, priority: priority})
	m.cond.Signal()
	m.mu.Unlock()
	return nil
}

func (m *MemoryQueue) Dequeue(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for m.heap.Len() == 0 && !m.done {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		m.cond.Wait()
	}
	if m.done && m.heap.Len() == 0 {
		return "", context.Canceled
	}
	item := heap.Pop(&m.heap).(*queueItem)
	return item.id, nil
}

func (m *MemoryQueue) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.heap.Len()
}

func (m *MemoryQueue) Close() error {
	m.mu.Lock()
	m.done = true
	m.cond.Broadcast()
	m.mu.Unlock()
	return nil
}
