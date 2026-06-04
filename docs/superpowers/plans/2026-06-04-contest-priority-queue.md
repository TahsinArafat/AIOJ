# Contest Priority Queue Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prioritize currently-running contest submissions over practice/gym/upsolving submissions in the judge queue.

**Architecture:**
1. Swap Redis queue from List (`LPush`/`BRPop`) to Sorted Set (`ZAdd`/`BZPopMin`). Contest submissions get score 0 (highest priority), practice/other get score 1 (lower priority).
2. Update the in-memory queue to use `container/heap` for priority ordering.
3. Add `priority int` parameter to the `JudgeQueue.Enqueue` interface and have the submission handler determine priority at enqueue time by checking if the contest is currently active.

**Tech Stack:** Go, Redis Sorted Sets, `container/heap`

---

### Task 1: Update JudgeQueue Interface and RedisQueue

**Files:**
- Modify: `internal/queue/interface.go`
- Modify: `internal/queue/redis.go`

- [ ] **Step 1: Add priority parameter to Enqueue interface**

In `internal/queue/interface.go`, update the `Enqueue` signature:

```go
type JudgeQueue interface {
	Enqueue(ctx context.Context, submissionID string, priority int) error
	Dequeue(ctx context.Context) (string, error)
	Len() int
	Close() error
}
```

- [ ] **Step 2: Replace Redis List with Sorted Set**

In `internal/queue/redis.go`, replace `LPush` with `ZAdd` and `BRPop` with `BZPopMin`:

```go
func (q *RedisQueue) Enqueue(ctx context.Context, id string, priority int) error {
	return q.client.ZAdd(ctx, q.queueName, redis.Z{
		Score:  float64(priority),
		Member: id,
	}).Err()
}

func (q *RedisQueue) Dequeue(ctx context.Context) (string, error) {
	result, err := q.client.BZPopMin(ctx, 2*time.Second, q.queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	return result.Member.(string), nil
}
```

- [ ] **Step 3: Update Len to use ZCard**

Replace `LLen` with `ZCard`:

```go
func (q *RedisQueue) Len() int {
	l, err := q.client.ZCard(context.Background(), q.queueName).Result()
	if err != nil {
		return 0
	}
	return int(l)
}
```

- [ ] **Step 4: Run tests to verify queue compiles**

Run: `go test ./internal/queue/...`
Expected: PASS

---

### Task 2: Update MemoryQueue with Heap-based Priority

**Files:**
- Modify: `internal/queue/memory.go`

- [ ] **Step 1: Implement priority queue using container/heap**

Replace the simple `[]string` slice with a heap-based priority queue in `internal/queue/memory.go`:

```go
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
	mu    sync.Mutex
	cond  *sync.Cond
	heap  priorityQueue
	done  bool
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
```

- [ ] **Step 2: Run tests to verify**

Run: `go test ./internal/queue/... -v`
Expected: PASS

---

### Task 3: Pass Priority from Submission Handler

**Files:**
- Modify: `internal/api/handler/submission.go`

- [ ] **Step 1: Determine priority in buildAndEnqueue**

In `internal/api/handler/submission.go`, locate `buildAndEnqueue` (around line 72). Before the enqueue call (line 99), add priority determination logic:

```go
	priority := 1
	if prob.Source != "" && prob.Source != "local" && h.vjudgeSvc != nil {
		// Remote problem — skip local queue
	} else {
		if req.ContestID != "" && !req.Upsolving {
			contest, cerr := h.contestStore.GetByID(r.Context(), req.ContestID)
			if cerr == nil && contest != nil {
				now := time.Now()
				if now.After(contest.StartTime) && now.Before(contest.EndTime) {
					priority = 0
				}
			}
		}
		h.queue.Enqueue(r.Context(), sub.ID, priority)
	}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./internal/...`
Expected: No errors

---

### Task 4: Update Test Callers

**Files:**
- Modify: `internal/queue/memory_test.go`

- [ ] **Step 1: Update all queue.Enqueue calls to pass priority**

If any test files call `Enqueue`, update them to pass the new `priority int` parameter (value `1` for default low priority, `0` for high priority test scenarios).

```go
// Example:
q.Enqueue(context.Background(), "test-id", 1)
```

- [ ] **Step 2: Run queue tests**

Run: `go test ./internal/queue/... -v`
Expected: PASS

---

### Task 5: Rebuild and Verify End-to-End

- [ ] **Step 1: Build all packages**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 2: Run all tests**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 3: Rebuild and restart judge-worker**

```bash
docker compose build judge-worker && docker compose restart judge-worker
```

Expected: Worker starts, dequeues contest submissions first.
