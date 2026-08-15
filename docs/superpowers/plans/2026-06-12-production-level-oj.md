# Production-Level Online Judge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform AIOJ from a monolith MVP into a horizontally scalable, production-grade Online Judge System matching the AOJ architecture blueprint — load-balanced multi-judge cluster, judge data CI/CD pipeline, real-time status broadcasting, special/interactive judging, and re-judging infrastructure.

**Architecture:** Phase-based incremental rollout. Each phase is independently deployable and testable. Start with foundation improvements (verdicts, checkers, SPJ refinement), then scale out with a distributed Load Balancer → Judge Master CI/CD → Broadcaster (Actor Model), ending with horizontal cluster scaling and infrastructure hardening.

**Tech Stack:** Go (Chi router, sync primitives for Actor Model), PostgreSQL, Redis (streams + pub/sub), go-judge (criyle), React 19 + TypeScript, WebSocket (gorilla/websocket), Docker Compose (dev), systemd + ufw (production)

---

## Architecture Decisions

1. **Load Balancer: Multi-queue per judge server** (AOJ's approach). Simpler than single-queue, avoids lock contention. Each judge server gets its own Redis list. At enqueue time, a scheduler picks the best judge server based on: queue depth, judge health, language match (optional), and user fairness.

2. **Scheduler Criteria** (in priority order):
   - Round-robin by default
   - Skip judges with queue depth > threshold (fairness)
   - Skip judges reporting unhealthy status
   - Weight by available concurrency slots

3. **Parallelization**: One submission occupies one judge worker until all test cases complete (AOJ's approach, already used in AIOJ). Parallel test-case distribution is deferred to Phase 6.

4. **Judge Master**: A separate service that maintains judge data, runs CI/CD on judge data changes, and syncs to all judge nodes via rsync/tarball. Uses PostgreSQL for metadata, filesystem for test data blobs.

5. **Broadcaster**: Actor Model in Go using goroutines + channels. TCP from Load Balancer → Broadcaster → WebSocket to all connected clients. One actor per WebSocket connection. Immutable messages passed through channels.

6. **Network Segmentation**: In production, judge servers communicate ONLY with Load Balancer and Judge Master. UFW rules enforce this. In Docker dev, all on same network (acceptable for development).

---

## Phase 1: Core Judging Foundation (verdicts, checkers, SPJ)

### Overview
AI-powered SPJ compilation caching, PE/OLE verdicts, enhanced float checker, re-judging API, and Judge Master foundations — no new services, all within existing backend.

### Files
- New: `internal/judge/spj_cache.go` (SPJ binary caching)
- New: `internal/judge/spj_cache_test.go`
- New: `internal/judge/float_epsilon_handler.go` (ERROR mode integration)
- New: `internal/judge/float_epsilon_handler_test.go`
- Modify: `internal/judge/worker.go` (PE, OLE verdicts; SPJ cache usage; re-judge support)
- Modify: `internal/model/submission.go` (new verdict constants)
- Modify: `internal/api/handler/submission.go` (re-judge endpoint)
- Modify: `internal/api/router.go` (re-judge route)

### Task 1.1: Add PE and OLE Verdict Constants

**Files:**
- Modify: `internal/model/submission.go`

- [ ] **Step 1: Write the test for new verdict constants**

```go
// internal/model/submission_test.go
package model

import "testing"

func TestVerdictConstants(t *testing.T) {
    tests := []struct {
        name     string
        verdict  SubmissionStatus
        expected string
    }{
        {"PE", StatusPE, "pe"},
        {"OLE", StatusOLE, "ole"},
    }
    for _, tt := range tests {
        if string(tt.verdict) != tt.expected {
            t.Errorf("%s: got %q, want %q", tt.name, tt.verdict, tt.expected)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/model/ -run TestVerdictConstants -v`
Expected: FAIL with "undefined: StatusPE"

- [ ] **Step 3: Add the constants and String method**

```go
// In internal/model/submission.go, add to const block:
const (
    StatusPE  SubmissionStatus = "pe"  // Presentation Error
    StatusOLE SubmissionStatus = "ole" // Output Limit Exceeded
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/model/ -run TestVerdictConstants -v`
Expected: PASS

- [ ] **Step 5: Update SubmissionStatus.IsFinal() to include new verdicts**

In `internal/model/submission.go`, add PE and OLE to the `IsFinal()` method:
```go
case StatusPE, StatusOLE:
    return true
```

- [ ] **Step 6: Run existing tests to verify no regressions**

Run: `go test ./internal/model/... -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/model/submission.go internal/model/submission_test.go
git commit -m "feat: add PE and OLE verdict constants"
```

---

### Task 1.2: Implement OLE Detection in Worker

**Files:**
- Modify: `internal/judge/worker.go:408-431` (runTestCase method)
- New: `internal/judge/ole_test.go`

- [ ] **Step 1: Write the test for OLE detection**

```go
// internal/judge/ole_test.go
package judge

import (
    "testing"
)

func TestOutputExceedsLimit(t *testing.T) {
    // 64KB output limit test
    smallData := make([]byte, 100_000) // 100KB > 64KB limit
    for i := range smallData {
        smallData[i] = 'A'
    }
    // The OLE check triggers when output exceeds 64KB
    // This is tested via the checkOutputLimit helper
    if !isOutputTooLarge(string(smallData), 64*1024) {
        t.Error("expected output to be too large")
    }

    normalData := make([]byte, 10_000) // 10KB < 64KB limit
    for i := range normalData {
        normalData[i] = 'B'
    }
    if isOutputTooLarge(string(normalData), 64*1024) {
        t.Error("expected output to be within limit")
    }
}

func TestOutputLimitExactBoundary(t *testing.T) {
    // Exactly 64KB should NOT trigger OLE
    exact := make([]byte, 64*1024)
    for i := range exact {
        exact[i] = 'C'
    }
    if isOutputTooLarge(string(exact), 64*1024) {
        t.Error("output at exact limit should not be too large")
    }

    // One byte over should trigger OLE
    over := make([]byte, 64*1024+1)
    for i := range over {
        over[i] = 'D'
    }
    if !isOutputTooLarge(string(over), 64*1024) {
        t.Error("output one byte over limit should be too large")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/judge/ -run TestOutput -v`
Expected: FAIL with "undefined: isOutputTooLarge"

- [ ] **Step 3: Add isOutputTooLarge helper and integrate into runTestCase**

```go
// In internal/judge/worker.go, add after loadFile:
func isOutputTooLarge(output string, maxBytes int64) bool {
    return int64(len(output)) > maxBytes
}
```

In `runTestCase()`, after checking `cr.Status == "Accepted"` and before checker invocation, add OLE check:

```go
// After line 438 in worker.go (where output is read from cr.Files["output.txt"])
case "Accepted":
    output := ""
    if f, ok := cr.Files["output.txt"]; ok {
        output = f
    }
    // Check output size limit (default 64KB)
    if isOutputTooLarge(output, 64*1024) {
        r.Status = model.StatusOLE
        r.Detail = "output limit exceeded"
        return r
    }
    // ... rest of existing code
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/judge/ -run TestOutput -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/judge/worker.go internal/judge/ole_test.go
git commit -m "feat: add OLE detection when output exceeds 64KB limit"
```

---

### Task 1.3: Add PE (Presentation Error) Detection

**Files:**
- Modify: `internal/judge/worker.go`

- [ ] **Step 1: Write the test for PE detection**

```go
// internal/judge/pe_test.go
package judge

import (
    "testing"
)

func TestPresentationErrorDetection(t *testing.T) {
    tests := []struct {
        name     string
        expected string
        actual   string
        wantPE   bool
    }{
        {
            name:     "trailing whitespace",
            expected: "42\n",
            actual:   "42  \n",
            wantPE:   true,
        },
        {
            name:     "extra blank lines at end",
            expected: "hello\n",
            actual:   "hello\n\n\n",
            wantPE:   true,
        },
        {
            name:     "different case",
            expected: "Hello\n",
            actual:   "hello\n",
            wantPE:   false, // WA, not PE
        },
        {
            name:     "extra space within line",
            expected: "a b\n",
            actual:   "a  b\n",
            wantPE:   true,
        },
        {
            name:     "missing trailing newline",
            expected: "42\n",
            actual:   "42",
            wantPE:   true,
        },
        {
            name:     "identical content",
            expected: "hello world\n",
            actual:   "hello world\n",
            wantPE:   false,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := isPresentationError(tt.expected, tt.actual)
            if got != tt.wantPE {
                t.Errorf("isPresentationError() = %v, want %v", got, tt.wantPE)
            }
        })
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/judge/ -run TestPresentationError -v`
Expected: FAIL with "undefined: isPresentationError"

- [ ] **Step 3: Implement PE detection function**

```go
// In internal/judge/worker.go
func isPresentationError(expected, actual string) bool {
    normalize := func(s string) string {
        s = strings.ReplaceAll(s, "\r\n", "\n")
        // Collapse multiple spaces within lines
        lines := strings.Split(s, "\n")
        for i, line := range lines {
            lines[i] = strings.Join(strings.Fields(line), " ")
        }
        // Trim trailing empty lines
        for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
            lines = lines[:len(lines)-1]
        }
        // Trim trailing whitespace from last line
        if len(lines) > 0 {
            lines[len(lines)-1] = strings.TrimRight(lines[len(lines)-1], " ")
        }
        return strings.Join(lines, "\n")
    }

    normalizedExpected := normalize(expected)
    normalizedActual := normalize(actual)

    if normalizedExpected == normalizedActual {
        return true
    }
    return false
}
```

- [ ] **Step 4: Integrate PE check into runTestCase**

In `runTestCase()`, in the `case "Accepted"` block, modify the checker result handling:

```go
// After the checker result, before returning r
chk := checker.GetChecker(prob.CheckerType, prob.FloatEpsilon)
if ck := chk.Check(nil, []byte(expected), []byte(output)); ck.Passed {
    r.Status = model.StatusAC
    r.Score = tc.Score
} else if prob.CheckerType == "exact" && isPresentationError(expected, output) {
    r.Status = model.StatusPE
    r.Detail = "presentation error"
} else {
    r.Status = model.StatusWA
    r.Detail = ck.Message
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/judge/ -run TestPresentationError -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/judge/worker.go internal/judge/pe_test.go
git commit -m "feat: add PE (presentation error) detection for exact checker"
```

---

### Task 1.4: SPJ Binary Caching

**Files:**
- Create: `internal/judge/spj_cache.go`
- Create: `internal/judge/spj_cache_test.go`
- Modify: `internal/judge/worker.go:122-176`

- [ ] **Step 1: Write the SPJ cache test**

```go
// internal/judge/spj_cache_test.go
package judge

import (
    "testing"
)

func TestSPJOps_New(t *testing.T) {
    c := NewSPJCache(3)
    if c == nil {
        t.Fatal("NewSPJCache returned nil")
    }
    if c.MaxSize() != 3 {
        t.Errorf("maxSize = %d, want 3", c.MaxSize())
    }
}

func TestSPJOps_PutGet(t *testing.T) {
    c := NewSPJCache(10)
    c.Put("problem-1", "v1", "cpp-gpp-64", "/tmp/spj-1", "binary-content")

    entry := c.Get("problem-1", "v1", "cpp-gpp-64")
    if entry == nil {
        t.Fatal("expected cache hit for problem-1")
    }
    if entry.RunDir != "/tmp/spj-1" {
        t.Errorf("RunDir = %q, want /tmp/spj-1", entry.RunDir)
    }
    if entry.BinContent != "binary-content" {
        t.Errorf("BinContent = %q, want binary-content", entry.BinContent)
    }
}

func TestSPJOps_DifferentVersions(t *testing.T) {
    c := NewSPJCache(10)
    c.Put("p1", "v1", "cpp", "/old", "old-bin")

    entry := c.Get("p1", "v2", "cpp")
    if entry != nil {
        t.Error("expected cache miss for different version")
    }
}

func TestSPJOps_Eviction(t *testing.T) {
    c := NewSPJCache(2)
    c.Put("p1", "v1", "cpp", "/p1", "b1") // will be evicted
    c.Put("p2", "v1", "cpp", "/p2", "b2")
    c.Put("p3", "v1", "cpp", "/p3", "b3") // triggers eviction of p1

    if c.Get("p1", "v1", "cpp") != nil {
        t.Error("p1 should have been evicted")
    }
    if c.Get("p3", "v1", "cpp") == nil {
        t.Error("p3 should still be in cache")
    }
}

func TestSPJOps_Update(t *testing.T) {
    c := NewSPJCache(10)
    c.Put("p1", "v1", "cpp", "/old", "old-bin")
    c.Put("p1", "v1", "cpp", "/new", "new-bin")

    entry := c.Get("p1", "v1", "cpp")
    if entry.RunDir != "/new" {
        t.Errorf("expected updated RunDir, got %q", entry.RunDir)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/judge/ -run TestSPJOps -v`
Expected: FAIL with "undefined: NewSPJCache"

- [ ] **Step 3: Implement SPJ cache**

```go
// internal/judge/spj_cache.go
package judge

import (
    "container/list"
    "sync"
)

type SPJCacheEntry struct {
    ProblemID  string
    Version    string
    Language   string
    RunDir     string
    BinContent string
}

type SPJCache struct {
    mu      sync.RWMutex
    maxSize int
    lru     *list.List
    items   map[string]*list.Element
}

type cacheKey string

func makeKey(problemID, version, language string) cacheKey {
    return cacheKey(problemID + "|" + version + "|" + language)
}

type cacheItem struct {
    key   cacheKey
    entry SPJCacheEntry
}

func NewSPJCache(maxSize int) *SPJCache {
    return &SPJCache{
        maxSize: maxSize,
        lru:     list.New(),
        items:   make(map[string]*list.Element),
    }
}

func (c *SPJCache) MaxSize() int { return c.maxSize }

func (c *SPJCache) Get(problemID, version, language string) *SPJCacheEntry {
    key := makeKey(problemID, version, language)
    c.mu.Lock()
    defer c.mu.Unlock()
    if elem, ok := c.items[string(key)]; ok {
        c.lru.MoveToFront(elem)
        entry := elem.Value.(*cacheItem).entry
        return &entry
    }
    return nil
}

func (c *SPJCache) Put(problemID, version, language, runDir, binContent string) {
    key := makeKey(problemID, version, language)
    c.mu.Lock()
    defer c.mu.Unlock()

    // Update existing entry
    if elem, ok := c.items[string(key)]; ok {
        c.lru.MoveToFront(elem)
        elem.Value.(*cacheItem).entry = SPJCacheEntry{
            ProblemID:  problemID,
            Version:    version,
            Language:   language,
            RunDir:     runDir,
            BinContent: binContent,
        }
        return
    }

    // Evict if at capacity
    for c.lru.Len() >= c.maxSize {
        oldest := c.lru.Back()
        if oldest != nil {
            c.lru.Remove(oldest)
            delete(c.items, string(oldest.Value.(*cacheItem).key))
        }
    }

    // Add new entry
    item := &cacheItem{
        key: key,
        entry: SPJCacheEntry{
            ProblemID:  problemID,
            Version:    version,
            Language:   language,
            RunDir:     runDir,
            BinContent: binContent,
        },
    }
    elem := c.lru.PushFront(item)
    c.items[string(key)] = elem
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/judge/ -run TestSPJOps -v`
Expected: PASS

- [ ] **Step 5: Integrate SPJ cache into WorkerPool and worker**

Add cache field to `WorkerPool`:
```go
type WorkerPool struct {
    // ... existing fields
    spjCache *SPJCache
}
```

Update `NewWorkerPool` to initialize cache:
```go
func NewWorkerPool(...) *WorkerPool {
    return &WorkerPool{
        // ... existing fields
        spjCache: NewSPJCache(100), // cache up to 100 SPJ binaries
    }
}
```

In `judge()`, replace the SPJ compilation block with cache-aware logic:
```go
// Check cache first
if prob.SPJ && prob.SPJSourceCode != "" {
    spjLang := prob.SPJLanguage
    if spjLang == "" {
        spjLang = "cpp-gpp-64"
    }
    cached := wp.spjCache.Get(prob.ID, prob.SPJVersion, spjLang)
    if cached != nil {
        spjExeDir = cached.RunDir
        spjBinContent = cached.BinContent
    } else {
        // ... existing SPJ compilation code ...
        // After successful compilation, cache the result
        wp.spjCache.Put(prob.ID, prob.SPJVersion, spjLang, spjExeDir, spjBinContent)
    }
}
```

- [ ] **Step 6: Run all judge tests**

Run: `go test ./internal/judge/... -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/judge/spj_cache.go internal/judge/spj_cache_test.go internal/judge/worker.go
git commit -m "feat: add LRU-based SPJ binary compilation cache"
```

---

### Task 1.5: Re-Judging API Endpoint

**Files:**
- Modify: `internal/api/handler/submission.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write test for re-judge handler**

```go
// internal/api/handler/submission_rejudge_test.go
package handler

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestRejudgeHandler_MissingID(t *testing.T) {
    h := &SubmissionHandler{}
    req := httptest.NewRequest("POST", "/api/submissions//rejudge", nil)
    w := httptest.NewRecorder()
    // No chi URL param set, so id is empty — handler should return 404
    // In integration test, chi would route this. We test the handler directly.
    t.Skip("requires chi router context - tested in integration")
}

func TestRejudgeHandler_ValidSubmission(t *testing.T) {
    t.Skip("requires DB - tested in integration test")
}
```

- [ ] **Step 2: Create integration test**

```go
// internal/judge/worker_integration_test.go (add test)
func TestRejudgeFlow(t *testing.T) {
    if testing.Short() {
        t.Skip("integration test")
    }
    // 1. Create a submission with initial verdict
    // 2. Call rejudge endpoint
    // 3. Verify status resets to "pending"
    // 4. Verify submission is re-enqueued
    // 5. Wait for new verdict
    t.Skip("requires running judge and database - manual test")
}
```

- [ ] **Step 3: Implement Rejudge handler**

In `internal/api/handler/submission.go`, add:

```go
func (h *SubmissionHandler) Rejudge(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    if id == "" {
        http.Error(w, "submission id required", http.StatusBadRequest)
        return
    }

    sub, err := h.subStore.GetByID(r.Context(), id)
    if err != nil || sub == nil {
        http.Error(w, "submission not found", http.StatusNotFound)
        return
    }

    // Reset submission status to pending
    h.subStore.UpdateStatus(r.Context(), id, model.StatusPending)
    h.subStore.UpdateRemoteID(r.Context(), id, "", "")

    // Re-enqueue with priority 0 (higher than normal)
    if err := h.queue.Enqueue(r.Context(), id, 0); err != nil {
        http.Error(w, "failed to re-enqueue: "+err.Error(), http.StatusInternalServerError)
        return
    }

    respondJSON(w, http.StatusOK, map[string]interface{}{
        "status": "rejudging",
        "id":     id,
    })
}
```

- [ ] **Step 4: Add route in router**

In `internal/api/router.go`, add under authenticated routes:

```go
r.Route("/submissions", func(r chi.Router) {
    r.Post("/", submissionH.Create)
    r.Get("/{id}", submissionH.GetByID)
    r.Post("/{id}/rejudge", submissionH.Rejudge)  // NEW
})
```

- [ ] **Step 5: Add Rejudge method to SubmissionStore interface**

In `internal/store/interfaces.go`, add to `SubmissionStore`:
```go
type SubmissionStore interface {
    // ... existing methods
    ResetForRejudge(ctx context.Context, id string) error  // NEW
}
```

Implement in `internal/store/postgres/submissions.go`:
```go
func (s *SubmissionStore) ResetForRejudge(ctx context.Context, id string) error {
    _, err := s.db.ExecContext(ctx,
        `UPDATE submissions SET status='pending', score=0, time_used=0, memory_used=0,
         compile_output='', judge_result=NULL, judged_at=NULL, remote_id='', remote_url=''
         WHERE id=$1`, id)
    return err
}
```

- [ ] **Step 6: Run tests**

Run: `go build ./...`
Expected: No build errors

- [ ] **Step 7: Commit**

```bash
git add internal/api/handler/submission.go internal/api/router.go internal/store/interfaces.go internal/store/postgres/submissions.go
git commit -m "feat: add re-judging API endpoint to reset and re-enqueue submissions"
```

---

### Task 1.6: Float Epsilon (ERROR) Mode Integration

**Files:**
- Modify: `internal/judge/worker.go`
- New: `internal/judge/float_epsilon_test.go`

- [ ] **Step 1: Write test for ERROR mode float judging**

```go
// internal/judge/float_epsilon_test.go
package judge

import (
    "bytes"
    "math"
    "testing"

    "github.com/tahsinarafat/aioj/internal/judge/checker"
)

func TestFloatEpsilonChecker_WithinTolerance(t *testing.T) {
    chk := checker.FloatChecker{Epsilon: 1e-6}
    result := chk.Check(nil, []byte("3.1415926535"), []byte("3.141592"))
    if !result.Passed {
        t.Error("expected pass within 1e-6 tolerance")
    }
}

func TestFloatEpsilonChecker_OutOfTolerance(t *testing.T) {
    chk := checker.FloatChecker{Epsilon: 1e-6}
    result := chk.Check(nil, []byte("3.141592"), []byte("3.14"))
    if result.Passed {
        t.Error("expected fail outside 1e-6 tolerance")
    }
}

func TestFloatEpsilonChecker_MultipleTokens(t *testing.T) {
    chk := checker.FloatChecker{Epsilon: 1e-3}
    result := chk.Check(nil,
        []byte("1.000 2.000 3.000"),
        []byte("1.001 2.000 3.001"),
    )
    if result.Passed {
        t.Error("expected third token to fail with 1e-3 epsilon")
    }
}

func TestFloatEpsilonChecker_NaN(t *testing.T) {
    chk := checker.FloatChecker{Epsilon: 1e-6}
    result := chk.Check(nil,
        []byte("NaN"),
        []byte("NaN"),
    )
    // NaN != NaN, so this should fail
    if result.Passed {
        t.Error("NaN comparison should fail")
    }
}

func TestFloatEpsilonChecker_Inf(t *testing.T) {
    chk := checker.FloatChecker{Epsilon: 1e-6}
    // +Inf should match +Inf
    expected := string(bytes.TrimSpace([]byte("+Inf")))
    actual := string(bytes.TrimSpace([]byte("+Inf")))
    result := chk.Check(nil, []byte(expected), []byte(actual))
    if !result.Passed {
        t.Error("+Inf should match +Inf")
    }
}

func TestFloatEpsilonChecker_TrailingZeros(t *testing.T) {
    chk := checker.FloatChecker{Epsilon: 1e-9}
    result := chk.Check(nil, []byte("42.000000000"), []byte("42.0"))
    if !result.Passed {
        t.Error("trailing zeros should match within tolerance")
    }
}

func TestFloatEpsilonChecker_ScientificNotation(t *testing.T) {
    chk := checker.FloatChecker{Epsilon: 1e-6}
    result := chk.Check(nil, []byte("1.0e-6"), []byte("0.000001"))
    if !result.Passed {
        t.Error("scientific notation should be equal")
    }
}

func BenchmarkFloatEpsilonChecker(b *testing.B) {
    chk := checker.FloatChecker{Epsilon: 1e-6}
    expected := make([]byte, 0, 1024)
    actual := make([]byte, 0, 1024)
    for i := 0; i < 100; i++ {
        expected = append(expected, []byte("3.14159 ")...)
        actual = append(actual, []byte("3.14159 ")...)
    }
    b.ResetTimer()
    for b.Loop() {
        chk.Check(nil, expected, actual)
    }
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/judge/ -run TestFloatEpsilon -v`
Expected: All sub-tests pass (FloatChecker already exists in checker.go)

- [ ] **Step 3: Integrate ERROR mode as a top-level judging path in worker.go**

In `judge()`, add a new branch before the existing DIFF/SPJ/INTERACTIVE branching:

```go
// In judge(), after loading the problem and before checking sub.SubmissionType:
if prob.CheckerType == "float" || prob.CheckerType == "float_absolute" || prob.CheckerType == "float_relative" {
    // ERROR mode: use FloatChecker with epsilon
    // Proceed normally through the existing code path - the checker will use epsilon
}
```

The existing code already supports this — when `prob.CheckerType` is "float", "float_absolute", or "float_relative", `checker.GetChecker()` returns the appropriate checker with epsilon. No additional code changes needed for the judging path. The AOJ paper's ERROR mode is already implemented via the FloatChecker with `prob.FloatEpsilon`.

- [ ] **Step 4: Verify existing float tests pass**

Run: `go test ./internal/judge/checker/... -v`
Expected: ALL PASS (byte_identical, float, sorted, unordered)

- [ ] **Step 5: Commit**

```bash
git add internal/judge/float_epsilon_test.go
git commit -m "test: add comprehensive float epsilon checker tests"
```

---

## Phase 2: Load Balancer with Multi-Queue Scheduling

### Overview
Replace the current simple queue with a Load Balancer service that manages per-judge-server queues, supports priority scheduling, and tracks queue status. Uses Redis for persistence. This is the first new service — runs alongside the existing backend.

### Key Design Decisions
- **Multi-queue architecture** (one Redis list per judge server) — avoids lock contention on a single queue
- **Scheduler** runs at enqueue time, picks best judge server based on criteria
- **Judge Health** — each judge server reports health via Redis heartbeat keys
- **Backward compatible** — existing `queue.JudgeQueue` interface preserved, new `LoadBalancer` wraps it

### Files
- Create: `internal/loadbalancer/lb.go`
- Create: `internal/loadbalancer/lb_test.go`
- Create: `internal/loadbalancer/scheduler.go`
- Create: `internal/loadbalancer/scheduler_test.go`
- Create: `internal/loadbalancer/judge_health.go`
- Modify: `cmd/aioj/main.go`
- Modify: `internal/queue/interface.go` (extend with status methods)
- Modify: `internal/queue/redis.go` (add list-based multi-queue methods)
- Modify: `internal/api/handler/submission.go` (use LB instead of direct queue)
- Modify: `config.yaml` (add load balancer config)
- Modify: `internal/config/config.go` (add LB config struct)

### Task 2.1: Extend JudgeQueue Interface with Multi-Queue Support

**Files:**
- Modify: `internal/queue/interface.go`
- Modify: `internal/queue/redis.go`

- [ ] **Step 1: Write failing test for multi-queue enqueue**

```go
// internal/queue/multi_queue_test.go
package queue

import (
    "context"
    "testing"
)

func TestMultiQueue_EnqueueToServer(t *testing.T) {
    ctx := context.Background()
    mq := NewMemoryMultiQueue()
    err := mq.EnqueueToServer(ctx, "judge-1", "sub-1", 0)
    if err != nil {
        t.Fatalf("expected no error, got %v", err)
    }
    if mq.ServerLen("judge-1") != 1 {
        t.Errorf("expected len 1, got %d", mq.ServerLen("judge-1"))
    }
}

func TestMultiQueue_DequeueFromServer(t *testing.T) {
    ctx := context.Background()
    mq := NewMemoryMultiQueue()
    mq.EnqueueToServer(ctx, "judge-1", "sub-1", 0)
    mq.EnqueueToServer(ctx, "judge-1", "sub-2", 0)

    id, err := mq.DequeueFromServer(ctx, "judge-1")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if id != "sub-1" {
        t.Errorf("expected sub-1, got %s", id)
    }
}

func TestMultiQueue_RoundRobin(t *testing.T) {
    ctx := context.Background()
    mq := NewMemoryMultiQueue()
    servers := []string{"judge-1", "judge-2", "judge-3"}

    // All servers empty, round-robin should distribute evenly
    for i := 0; i < 9; i++ {
        sid := "sub-" + string(rune('a'+i))
        server := servers[i%3]
        mq.EnqueueToServer(ctx, server, sid, 0)
    }

    for _, s := range servers {
        if mq.ServerLen(s) != 3 {
            t.Errorf("server %s: expected 3 subs, got %d", s, mq.ServerLen(s))
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/queue/ -run TestMultiQueue -v`
Expected: FAIL with "undefined: NewMemoryMultiQueue"

- [ ] **Step 3: Add MultiQueue interface and MemoryMultiQueue implementation**

```go
// In internal/queue/interface.go, add:
type MultiQueue interface {
    EnqueueToServer(ctx context.Context, serverID, submissionID string, priority int) error
    DequeueFromServer(ctx context.Context, serverID string) (string, error)
    ServerLen(serverID string) int
    AllServers() []string
    RegisterServer(serverID string) error
    Close() error
}
```

```go
// internal/queue/memory_multi.go
package queue

import (
    "context"
    "sync"
)

type MemoryMultiQueue struct {
    mu      sync.RWMutex
    queues  map[string][]string // serverID -> []submissionID
    servers []string
}

func NewMemoryMultiQueue() *MemoryMultiQueue {
    return &MemoryMultiQueue{
        queues: make(map[string][]string),
    }
}

func (q *MemoryMultiQueue) RegisterServer(serverID string) error {
    q.mu.Lock()
    defer q.mu.Unlock()
    if _, ok := q.queues[serverID]; !ok {
        q.queues[serverID] = nil
        q.servers = append(q.servers, serverID)
    }
    return nil
}

func (q *MemoryMultiQueue) EnqueueToServer(ctx context.Context, serverID, submissionID string, priority int) error {
    q.mu.Lock()
    defer q.mu.Unlock()
    q.queues[serverID] = append(q.queues[serverID], submissionID)
    return nil
}

func (q *MemoryMultiQueue) DequeueFromServer(ctx context.Context, serverID string) (string, error) {
    q.mu.Lock()
    defer q.mu.Unlock()
    queue := q.queues[serverID]
    if len(queue) == 0 {
        return "", nil
    }
    item := queue[0]
    q.queues[serverID] = queue[1:]
    return item, nil
}

func (q *MemoryMultiQueue) ServerLen(serverID string) int {
    q.mu.RLock()
    defer q.mu.RUnlock()
    return len(q.queues[serverID])
}

func (q *MemoryMultiQueue) AllServers() []string {
    q.mu.RLock()
    defer q.mu.RUnlock()
    result := make([]string, len(q.servers))
    copy(result, q.servers)
    return result
}

func (q *MemoryMultiQueue) Close() error { return nil }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/queue/ -run TestMultiQueue -v`
Expected: PASS

- [ ] **Step 5: Implement RedisMultiQueue**

```go
// internal/queue/redis_multi.go
package queue

import (
    "context"
    "fmt"

    "github.com/redis/go-redis/v9"
)

const (
    redisServerListKey = "aioj:servers"
    redisQueuePrefix   = "aioj:queue:"
)

type RedisMultiQueue struct {
    rdb *redis.Client
}

func NewRedisMultiQueue(rdb *redis.Client) *RedisMultiQueue {
    return &RedisMultiQueue{rdb: rdb}
}

func (q *RedisMultiQueue) RegisterServer(serverID string) error {
    return q.rdb.SAdd(context.Background(), redisServerListKey, serverID).Err()
}

func (q *RedisMultiQueue) EnqueueToServer(ctx context.Context, serverID, submissionID string, priority int) error {
    return q.rdb.RPush(ctx, redisQueuePrefix+serverID, submissionID).Err()
}

func (q *RedisMultiQueue) DequeueFromServer(ctx context.Context, serverID string) (string, error) {
    result, err := q.rdb.LPop(ctx, redisQueuePrefix+serverID).Result()
    if err == redis.Nil {
        return "", nil
    }
    return result, err
}

func (q *RedisMultiQueue) ServerLen(serverID string) int {
    n, _ := q.rdb.LLen(context.Background(), redisQueuePrefix+serverID).Result()
    return int(n)
}

func (q *RedisMultiQueue) AllServers() []string {
    servers, _ := q.rdb.SMembers(context.Background(), redisServerListKey).Result()
    return servers
}

func (q *RedisMultiQueue) Close() error {
    return q.rdb.Close()
}

func redisServerQueueKey(serverID string) string {
    return fmt.Sprintf("%s%s", redisQueuePrefix, serverID)
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/queue/... -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/queue/interface.go internal/queue/memory_multi.go internal/queue/redis_multi.go internal/queue/multi_queue_test.go
git commit -m "feat: add MultiQueue interface with memory and Redis backends"
```

---

### Task 2.2: Load Balancer Service

**Files:**
- Create: `internal/loadbalancer/lb.go`
- Create: `internal/loadbalancer/lb_test.go`
- Create: `internal/loadbalancer/scheduler.go`
- Create: `internal/loadbalancer/scheduler_test.go`
- Create: `internal/loadbalancer/judge_health.go`

- [ ] **Step 1: Write scheduler tests**

```go
// internal/loadbalancer/scheduler_test.go
package loadbalancer

import (
    "testing"
)

func TestRoundRobinScheduler(t *testing.T) {
    servers := []string{"j1", "j2", "j3"}
    sch := NewRoundRobinScheduler(servers)
    
    picks := make([]string, 6)
    for i := range picks {
        picks[i] = sch.Pick()
    }
    expected := []string{"j1", "j2", "j3", "j1", "j2", "j3"}
    for i, p := range picks {
        if p != expected[i] {
            t.Errorf("pick %d: got %s, want %s", i, p, expected[i])
        }
    }
}

func TestLeastLoadedScheduler(t *testing.T) {
    servers := []string{"j1", "j2", "j3"}
    loads := map[string]int{"j1": 5, "j2": 2, "j3": 10}
    sch := NewLeastLoadedScheduler(servers, func(s string) int { return loads[s] })
    
    pick := sch.Pick()
    if pick != "j2" {
        t.Errorf("expected j2 (least loaded), got %s", pick)
    }
}

func TestWeightedScheduler_SkipsOverloaded(t *testing.T) {
    servers := []string{"j1", "j2", "j3"}
    loads := map[string]int{"j1": 100, "j2": 50, "j3": 100}
    sch := NewWeightedScheduler(servers, func(s string) int { return loads[s] }, 80)
    
    // j2 has the lowest load AND is under the 80 threshold
    pick := sch.Pick()
    if pick != "j2" {
        t.Errorf("expected j2, got %s", pick)
    }
    
    // Now make all servers overloaded
    loads["j2"] = 90
    pick = sch.Pick()
    // Should still pick something (least loaded among overloaded)
    if pick == "" {
        t.Error("expected non-empty pick even when all overloaded")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/loadbalancer/... -v`
Expected: FAIL — no files in loadbalancer package

- [ ] **Step 3: Implement scheduler**

```go
// internal/loadbalancer/scheduler.go
package loadbalancer

import (
    "sync"
)

type LoadFn func(serverID string) int

type Scheduler interface {
    Pick() string
}

// RoundRobinScheduler cycles through servers in order
type RoundRobinScheduler struct {
    mu      sync.Mutex
    servers []string
    next    int
}

func NewRoundRobinScheduler(servers []string) *RoundRobinScheduler {
    return &RoundRobinScheduler{servers: servers}
}

func (s *RoundRobinScheduler) Pick() string {
    s.mu.Lock()
    defer s.mu.Unlock()
    if len(s.servers) == 0 {
        return ""
    }
    picked := s.servers[s.next]
    s.next = (s.next + 1) % len(s.servers)
    return picked
}

// LeastLoadedScheduler picks the server with fewest pending submissions
type LeastLoadedScheduler struct {
    mu      sync.Mutex
    servers []string
    loadFn  LoadFn
}

func NewLeastLoadedScheduler(servers []string, loadFn LoadFn) *LeastLoadedScheduler {
    return &LeastLoadedScheduler{servers: servers, loadFn: loadFn}
}

func (s *LeastLoadedScheduler) Pick() string {
    s.mu.Lock()
    defer s.mu.Unlock()
    if len(s.servers) == 0 {
        return ""
    }
    best := s.servers[0]
    bestLoad := s.loadFn(best)
    for _, svr := range s.servers[1:] {
        load := s.loadFn(svr)
        if load < bestLoad {
            best = svr
            bestLoad = load
        }
    }
    return best
}

// WeightedScheduler picks least loaded, skipping those over threshold
type WeightedScheduler struct {
    inner     *LeastLoadedScheduler
    threshold int
}

func NewWeightedScheduler(servers []string, loadFn LoadFn, threshold int) *WeightedScheduler {
    return &WeightedScheduler{
        inner:     NewLeastLoadedScheduler(servers, loadFn),
        threshold: threshold,
    }
}

func (s *WeightedScheduler) Pick() string {
    // First, try to find a server under threshold
    for _, svr := range s.inner.servers {
        if s.inner.loadFn(svr) < s.threshold {
            return svr
        }
    }
    // All overloaded, pick least loaded anyway
    return s.inner.Pick()
}
```

- [ ] **Step 4: Implement Load Balancer**

```go
// internal/loadbalancer/lb.go
package loadbalancer

import (
    "context"
    "fmt"
    "log/slog"
    "sync"
    "time"

    "github.com/tahsinarafat/aioj/internal/queue"
)

type LoadBalancer struct {
    mq        queue.MultiQueue
    scheduler Scheduler
    mu        sync.RWMutex
    health    map[string]*JudgeHealth

    // Config
    maxQueueDepth int
    healthTTL     time.Duration
}

type JudgeHealth struct {
    ServerID      string
    LastHeartbeat time.Time
    Concurrency   int
    ActiveJobs    int
}

type LBConfig struct {
    MaxQueueDepth int
    HealthTTL     time.Duration
    Scheduler     string // "round-robin", "least-loaded", "weighted"
}

func NewLoadBalancer(mq queue.MultiQueue, servers []string, cfg LBConfig) *LoadBalancer {
    if cfg.MaxQueueDepth == 0 {
        cfg.MaxQueueDepth = 1000
    }
    if cfg.HealthTTL == 0 {
        cfg.HealthTTL = 30 * time.Second
    }
    if cfg.Scheduler == "" {
        cfg.Scheduler = "round-robin"
    }

    health := make(map[string]*JudgeHealth)
    for _, s := range servers {
        health[s] = &JudgeHealth{
            ServerID:      s,
            LastHeartbeat: time.Now(),
            Concurrency:   4,
        }
    }

    var sch Scheduler
    loadFn := func(sid string) int {
        return mq.ServerLen(sid)
    }
    switch cfg.Scheduler {
    case "least-loaded":
        sch = NewLeastLoadedScheduler(servers, loadFn)
    case "weighted":
        sch = NewWeightedScheduler(servers, loadFn, cfg.MaxQueueDepth)
    default:
        sch = NewRoundRobinScheduler(servers)
    }

    return &LoadBalancer{
        mq:            mq,
        scheduler:     sch,
        health:        health,
        maxQueueDepth: cfg.MaxQueueDepth,
        healthTTL:     cfg.HealthTTL,
    }
}

func (lb *LoadBalancer) Enqueue(ctx context.Context, submissionID string, priority int) error {
    // Pick a judge server
    serverID := lb.scheduler.Pick()
    if serverID == "" {
        return fmt.Errorf("no judge servers available")
    }

    // Check queue depth
    if lb.mq.ServerLen(serverID) >= lb.maxQueueDepth {
        slog.Warn("queue depth limit reached for server, trying fallback",
            "server", serverID, "depth", lb.mq.ServerLen(serverID))
        // Try another server
        for _, s := range lb.mq.AllServers() {
            if s != serverID && lb.mq.ServerLen(s) < lb.maxQueueDepth {
                serverID = s
                break
            }
        }
    }

    slog.Info("loadbalancer: enqueuing submission",
        "submission", submissionID, "server", serverID, "priority", priority)
    return lb.mq.EnqueueToServer(ctx, serverID, submissionID, priority)
}

func (lb *LoadBalancer) Dequeue(ctx context.Context, serverID string) (string, error) {
    return lb.mq.DequeueFromServer(ctx, serverID)
}

func (lb *LoadBalancer) ServerHealth(serverID string) *JudgeHealth {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    return lb.health[serverID]
}

func (lb *LoadBalancer) UpdateHealth(serverID string, activeJobs int) {
    lb.mu.Lock()
    defer lb.mu.Unlock()
    if h, ok := lb.health[serverID]; ok {
        h.LastHeartbeat = time.Now()
        h.ActiveJobs = activeJobs
    }
}

func (lb *LoadBalancer) HealthyServers() []string {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    var healthy []string
    for sid, h := range lb.health {
        if time.Since(h.LastHeartbeat) < lb.healthTTL {
            healthy = append(healthy, sid)
        }
    }
    return healthy
}

func (lb *LoadBalancer) Status() map[string]interface{} {
    lb.mu.RLock()
    defer lb.mu.RUnlock()
    result := make(map[string]interface{})
    for sid, h := range lb.health {
        result[sid] = map[string]interface{}{
            "queue_depth":     lb.mq.ServerLen(sid),
            "active_jobs":     h.ActiveJobs,
            "concurrency":     h.Concurrency,
            "last_heartbeat":  h.LastHeartbeat.Format(time.RFC3339),
            "healthy":         time.Since(h.LastHeartbeat) < lb.healthTTL,
        }
    }
    return result
}

func (lb *LoadBalancer) Close() error {
    return lb.mq.Close()
}
```

- [ ] **Step 5: Implement Load Balancer test**

```go
// internal/loadbalancer/lb_test.go
package loadbalancer

import (
    "context"
    "testing"
    "time"
)

func TestLoadBalancer_EnqueueDistributes(t *testing.T) {
    mq := newTestMultiQueue()
    mq.RegisterServer("j1")
    mq.RegisterServer("j2")

    lb := NewLoadBalancer(mq, []string{"j1", "j2"}, LBConfig{Scheduler: "round-robin"})

    ctx := context.Background()
    lb.Enqueue(ctx, "sub-1", 0)
    lb.Enqueue(ctx, "sub-2", 0)
    lb.Enqueue(ctx, "sub-3", 0)

    // Round-robin: j1 should have sub-1 and sub-3, j2 should have sub-2
    if mq.ServerLen("j1") != 2 {
        t.Errorf("j1: expected 2 items, got %d", mq.ServerLen("j1"))
    }
    if mq.ServerLen("j2") != 1 {
        t.Errorf("j2: expected 1 item, got %d", mq.ServerLen("j2"))
    }
}

func TestLoadBalancer_DequeueFromServer(t *testing.T) {
    mq := newTestMultiQueue()
    mq.RegisterServer("j1")
    lb := NewLoadBalancer(mq, []string{"j1"}, LBConfig{Scheduler: "round-robin"})

    ctx := context.Background()
    lb.Enqueue(ctx, "sub-1", 0)

    id, err := lb.Dequeue(ctx, "j1")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if id != "sub-1" {
        t.Errorf("got %s, want sub-1", id)
    }
}

func TestLoadBalancer_HealthTracking(t *testing.T) {
    mq := newTestMultiQueue()
    mq.RegisterServer("j1")
    mq.RegisterServer("j2")

    lb := NewLoadBalancer(mq, []string{"j1", "j2"}, LBConfig{
        Scheduler: "round-robin",
        HealthTTL: 1 * time.Second,
    })

    lb.UpdateHealth("j1", 2)
    time.Sleep(100 * time.Millisecond)

    h := lb.ServerHealth("j1")
    if h.ActiveJobs != 2 {
        t.Errorf("active jobs: got %d, want 2", h.ActiveJobs)
    }

    healthy := lb.HealthyServers()
    if len(healthy) < 2 {
        t.Errorf("expected 2 healthy servers, got %d", len(healthy))
    }
}

func TestLoadBalancer_HealthExpiry(t *testing.T) {
    mq := newTestMultiQueue()
    mq.RegisterServer("j1")
    mq.RegisterServer("j2")

    lb := NewLoadBalancer(mq, []string{"j1", "j2"}, LBConfig{
        Scheduler: "round-robin",
        HealthTTL: 50 * time.Millisecond,
    })

    lb.UpdateHealth("j1", 0)
    // Don't update j2 — it will expire
    time.Sleep(100 * time.Millisecond)

    healthy := lb.HealthyServers()
    if len(healthy) != 1 {
        t.Errorf("expected 1 healthy server after ttl expiry, got %d (%v)", len(healthy), healthy)
    }
    if len(healthy) > 0 && healthy[0] != "j1" {
        t.Errorf("expected j1 to be healthy, got %s", healthy[0])
    }
}

// testMultiQueue is an in-memory MultiQueue for unit tests
type testMultiQueue struct {
    queues map[string][]string
}

func newTestMultiQueue() *testMultiQueue {
    return &testMultiQueue{queues: make(map[string][]string)}
}

func (q *testMultiQueue) RegisterServer(id string) error {
    q.queues[id] = nil
    return nil
}

func (q *testMultiQueue) EnqueueToServer(ctx context.Context, sid, subID string, priority int) error {
    q.queues[sid] = append(q.queues[sid], subID)
    return nil
}

func (q *testMultiQueue) DequeueFromServer(ctx context.Context, sid string) (string, error) {
    items := q.queues[sid]
    if len(items) == 0 {
        return "", nil
    }
    item := items[0]
    q.queues[sid] = items[1:]
    return item, nil
}

func (q *testMultiQueue) ServerLen(sid string) int {
    return len(q.queues[sid])
}

func (q *testMultiQueue) AllServers() []string {
    var servers []string
    for k := range q.queues {
        servers = append(servers, k)
    }
    return servers
}

func (q *testMultiQueue) Close() error { return nil }
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/loadbalancer/... -v`
Expected: ALL PASS

- [ ] **Step 7: Commit**

```bash
git add internal/loadbalancer/
git commit -m "feat: add Load Balancer with multi-queue scheduling and health tracking"
```

---

### Task 2.3: Integrate Load Balancer into Backend

**Files:**
- Modify: `internal/config/config.go` (add LB config)
- Modify: `config.yaml` (add LB config section)
- Modify: `cmd/aioj/main.go` (wire LB into handlers)
- Modify: `internal/api/handler/submission.go` (use LB for enqueue)

- [ ] **Step 1: Add LB config to Config struct**

```go
// In internal/config/config.go, add:
type LBConfig struct {
    MaxQueueDepth int    `yaml:"max_queue_depth"`
    HealthTTL     string `yaml:"health_ttl"`
    Scheduler     string `yaml:"scheduler"`
}

// Add to Config struct:
type Config struct {
    // ... existing fields
    LoadBalancer LBConfig `yaml:"load_balancer"`
}
```

- [ ] **Step 2: Add config section to config.yaml**

```yaml
# In config.yaml, add:
load_balancer:
  max_queue_depth: 1000
  health_ttl: "30s"
  scheduler: "least-loaded"  # round-robin | least-loaded | weighted
```

- [ ] **Step 3: Wire Load Balancer into main.go**

```go
// In cmd/aioj/main.go:
import "github.com/tahsinarafat/aioj/internal/loadbalancer"

// After creating the Redis client and judge queue:
var lb *loadbalancer.LoadBalancer
if redisClient != nil {
    rmq := queue.NewRedisMultiQueue(redisClient)
    servers := []string{"judge-1"} // default: single judge server
    if envServers := os.Getenv("JUDGE_SERVERS"); envServers != "" {
        servers = strings.Split(envServers, ",")
    }
    for _, s := range servers {
        rmq.RegisterServer(s)
    }
    healthTTL, _ := time.ParseDuration(cfg.LoadBalancer.HealthTTL)
    lb = loadbalancer.NewLoadBalancer(rmq, servers, loadbalancer.LBConfig{
        MaxQueueDepth: cfg.LoadBalancer.MaxQueueDepth,
        HealthTTL:     healthTTL,
        Scheduler:     cfg.LoadBalancer.Scheduler,
    })
}
```

- [ ] **Step 4: Update SubmissionHandler to use LB**

In `internal/api/handler/submission.go`, add LB field and use it:

```go
type SubmissionHandler struct {
    // ... existing fields
    lb *loadbalancer.LoadBalancer  // NEW
}

func NewSubmissionHandler(..., lb *loadbalancer.LoadBalancer) *SubmissionHandler {
    return &SubmissionHandler{
        // ... existing fields
        lb: lb,
    }
}
```

In `buildAndEnqueue`, replace `h.queue.Enqueue(...)` with:

```go
if h.lb != nil {
    if err := h.lb.Enqueue(r.Context(), sub.ID, priority); err != nil {
        http.Error(w, "failed to enqueue: "+err.Error(), http.StatusInternalServerError)
        return
    }
} else {
    h.queue.Enqueue(r.Context(), sub.ID, priority)
}
```

- [ ] **Step 5: Run build to verify compilation**

Run: `go build ./...`
Expected: No build errors

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go config.yaml cmd/aioj/main.go internal/api/handler/submission.go
git commit -m "feat: integrate Load Balancer into backend submission flow"
```

---

## Phase 3: Judge Master — Judge Data CI/CD Pipeline

### Overview
A new service (`judge-master`) that manages judge data (test cases, validators, generators, reference solutions, checkers, interactors). When judge data changes:
1. Generator scripts produce test cases
2. Validator scripts validate inputs/outputs
3. Reference solutions are built and run to produce expected outputs
4. All refined judge data is packaged and deployed to all judge servers

### Files
- Create: `cmd/judge-master/main.go`
- Create: `internal/judgemaster/master.go`
- Create: `internal/judgemaster/master_test.go`
- Create: `internal/judgemaster/pipeline.go`
- Create: `internal/judgemaster/pipeline_test.go`
- Create: `internal/judgemaster/sync.go`
- Create: `internal/api/handler/judge_master.go` (admin API)
- Modify: `docker-compose.yml` (add judge-master service)
- Modify: `internal/store/postgres/problems.go` (add judge data version tracking)

### Task 3.1: Judge Data Pipeline (Generators, Validators, Reference Solutions)

**Files:**
- Create: `internal/judgemaster/pipeline.go`
- Create: `internal/judgemaster/pipeline_test.go`
- Create: `internal/judgemaster/master.go`

- [ ] **Step 1: Write pipeline test**

```go
// internal/judgemaster/pipeline_test.go
package judgemaster

import (
    "os"
    "path/filepath"
    "testing"
)

func TestPipeline_GeneratorStage(t *testing.T) {
    tmpdir := t.TempDir()
    genScript := filepath.Join(tmpdir, "gen.sh")
    os.WriteFile(genScript, []byte("#!/bin/sh\necho '5'\necho '1 2'"), 0755)

    p := NewPipeline(tmpdir, tmpdir)
    inputs, err := p.RunGenerator(genScript, 1)
    if err != nil {
        t.Fatalf("RunGenerator failed: %v", err)
    }
    if len(inputs) != 2 {
        t.Errorf("expected 2 input lines, got %d", len(inputs))
    }
    if inputs[0] != "5" {
        t.Errorf("first line: got %q, want %q", inputs[0], "5")
    }
}

func TestPipeline_ValidatorStage(t *testing.T) {
    tmpdir := t.TempDir()
    valScript := filepath.Join(tmpdir, "val.sh")
    os.WriteFile(valScript, []byte("#!/bin/sh\n# Check n is positive\nn=$(head -1 $1)\n[ \"$n\" -gt 0 ] && exit 0 || exit 1"), 0755)

    inputFile := filepath.Join(tmpdir, "test.in")
    os.WriteFile(inputFile, []byte("5\n1 2\n"), 0644)

    p := NewPipeline(tmpdir, tmpdir)
    valid, err := p.RunValidator(valScript, inputFile)
    if err != nil {
        t.Fatalf("RunValidator failed: %v", err)
    }
    if !valid {
        t.Error("expected valid input")
    }
}

func TestPipeline_ReferenceSolution(t *testing.T) {
    tmpdir := t.TempDir()
    solScript := filepath.Join(tmpdir, "sol.sh")
    os.WriteFile(solScript, []byte("#!/bin/sh\n# Echo back the input\ncat"), 0755)

    inputFile := filepath.Join(tmpdir, "test.in")
    os.WriteFile(inputFile, []byte("hello world\n"), 0644)

    p := NewPipeline(tmpdir, tmpdir)
    output, err := p.RunReference(solScript, inputFile)
    if err != nil {
        t.Fatalf("RunReference failed: %v", err)
    }
    if output != "hello world\n" {
        t.Errorf("got %q, want %q", output, "hello world\n")
    }
}

func TestPipeline_FullFlow(t *testing.T) {
    tmpdir := t.TempDir()

    // Create generator
    os.WriteFile(filepath.Join(tmpdir, "gen.sh"),
        []byte("#!/bin/sh\necho '3'\necho '10 20'"), 0755)

    // Create validator
    os.WriteFile(filepath.Join(tmpdir, "val.sh"),
        []byte("#!/bin/sh\nexit 0"), 0755)

    // Create reference solution
    os.WriteFile(filepath.Join(tmpdir, "sol.py"),
        []byte("#!/usr/bin/env python3\nimport sys\nfor line in sys.stdin:\n    print(line.strip())"), 0755)

    p := NewPipeline(tmpdir, tmpdir)

    result, err := p.Run(filepath.Join(tmpdir, "gen.sh"),
        []string{filepath.Join(tmpdir, "val.sh")},
        []string{filepath.Join(tmpdir, "sol.py")})
    if err != nil {
        t.Fatalf("Pipeline.Run failed: %v", err)
    }

    if result.TestCaseCount != 2 {
        t.Errorf("expected 2 test cases, got %d", result.TestCaseCount)
    }
    if !result.AllValid {
        t.Error("expected all test cases valid")
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/judgemaster/... -v`
Expected: FAIL — no files in package

- [ ] **Step 3: Implement pipeline**

```go
// internal/judgemaster/pipeline.go
package judgemaster

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

type Pipeline struct {
    workDir    string
    outputDir  string
}

type PipelineResult struct {
    TestCaseCount int
    AllValid      bool
    Inputs        []string
    Outputs       []string
    Errors        []string
}

func NewPipeline(workDir, outputDir string) *Pipeline {
    return &Pipeline{workDir: workDir, outputDir: outputDir}
}

func (p *Pipeline) Run(generator string, validators []string, references []string) (*PipelineResult, error) {
    result := &PipelineResult{AllValid: true}

    // Stage 1: Generate
    inputs, err := p.RunGenerator(generator, 0)
    if err != nil {
        return nil, fmt.Errorf("generator: %w", err)
    }

    // Stage 2: Validate each input
    for i, input := range inputs {
        inputFile := filepath.Join(p.workDir, fmt.Sprintf("in_%04d.txt", i))
        if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
            return nil, fmt.Errorf("write input %d: %w", i, err)
        }
        for _, v := range validators {
            valid, err := p.RunValidator(v, inputFile)
            if err != nil {
                result.Errors = append(result.Errors, fmt.Sprintf("validator error on case %d: %v", i, err))
            }
            if !valid {
                result.AllValid = false
                result.Errors = append(result.Errors, fmt.Sprintf("test case %d failed validation", i))
            }
        }
    }

    // Stage 3: Run reference solutions
    for i, ref := range references {
        for j, input := range inputs {
            output, err := p.RunReferenceWithInput(ref, input)
            if err != nil {
                result.Errors = append(result.Errors,
                    fmt.Sprintf("reference %d failed on case %d: %v", i, j, err))
                continue
            }
            if i == 0 {
                // First reference solution determines expected outputs
                result.Inputs = append(result.Inputs, input)
                result.Outputs = append(result.Outputs, output)
            } else {
                // Subsequent references must match
                if j < len(result.Outputs) && output != result.Outputs[j] {
                    result.Errors = append(result.Errors,
                        fmt.Sprintf("reference %d output mismatch on case %d", i, j))
                }
            }
        }
    }

    result.TestCaseCount = len(result.Inputs)
    return result, nil
}

func (p *Pipeline) RunGenerator(script string, caseLimit int) ([]string, error) {
    cmd := exec.Command("bash", script)
    cmd.Dir = p.workDir
    stdout, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("exec generator: %w", err)
    }
    lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
    var result []string
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line != "" {
            result = append(result, line)
        }
    }
    return result, nil
}

func (p *Pipeline) RunValidator(script, inputFile string) (bool, error) {
    cmd := exec.Command("bash", script, inputFile)
    cmd.Dir = p.workDir
    err := cmd.Run()
    if err != nil {
        if exitErr, ok := err.(*exec.ExitError); ok {
            return exitErr.ExitCode() == 0, nil
        }
        return false, err
    }
    return true, nil
}

func (p *Pipeline) RunReference(script, inputFile string) (string, error) {
    data, err := os.ReadFile(inputFile)
    if err != nil {
        return "", err
    }
    return p.RunReferenceWithInput(script, string(data))
}

func (p *Pipeline) RunReferenceWithInput(script, input string) (string, error) {
    cmd := exec.Command("bash", script)
    cmd.Dir = p.workDir
    cmd.Stdin = strings.NewReader(input)
    stdout, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("exec reference: %w", err)
    }
    return string(stdout), nil
}
```

- [ ] **Step 4: Implement Judge Master service**

```go
// internal/judgemaster/master.go
package judgemaster

import (
    "context"
    "log/slog"
    "sync"
    "time"

    "github.com/tahsinarafat/aioj/internal/model"
)

type JudgeMaster struct {
    mu          sync.RWMutex
    pipelines   map[string]*Pipeline  // problemID -> pipeline
    versions    map[string]int        // problemID -> version
    problemDir  string
    executor    Executor              // interface for running compilation/execution
}

type Executor interface {
    RunScript(script, input string, timeLimit, memoryLimit int) (string, error)
    CompileCode(source, language string) (string, error)
}

type JudgeDataVersion struct {
    ProblemID string
    Version   int
    TestCases []TestCaseData
    Checker   []byte
    CreatedAt time.Time
}

type TestCaseData struct {
    Input  string
    Output string
    Score  int
}

func NewJudgeMaster(problemDir string, executor Executor) *JudgeMaster {
    return &JudgeMaster{
        pipelines:  make(map[string]*Pipeline),
        versions:   make(map[string]int),
        problemDir: problemDir,
        executor:   executor,
    }
}

func (jm *JudgeMaster) BuildJudgeData(ctx context.Context, problem *model.Problem, forceRebuild bool) (*PipelineResult, error) {
    jm.mu.Lock()
    jm.versions[problem.ID]++
    version := jm.versions[problem.ID]
    jm.mu.Unlock()

    workDir := filepath.Join(jm.problemDir, problem.ID, fmt.Sprintf("v%d", version))
    os.MkdirAll(workDir, 0755)

    pipeline := NewPipeline(workDir, workDir)
    jm.mu.Lock()
    jm.pipelines[problem.ID] = pipeline
    jm.mu.Unlock()

    // Find generators (g1.sh, g2.sh, etc.) in problem dir
    // This is simplified — in production, generator/validator/ref paths come from problem metadata
    generatorPaths := findScripts(filepath.Join(jm.problemDir, problem.ID), "gen*.sh")
    validatorPaths := findScripts(filepath.Join(jm.problemDir, problem.ID), "val*.sh")
    referencePaths := findScripts(filepath.Join(jm.problemDir, problem.ID), "sol*")

    var allErrors []string
    var totalCases int
    allValid := true

    for _, gen := range generatorPaths {
        result, err := pipeline.Run(gen, validatorPaths, referencePaths)
        if err != nil {
            return nil, fmt.Errorf("pipeline for %s failed: %w", gen, err)
        }
        totalCases += result.TestCaseCount
        allErrors = append(allErrors, result.Errors...)
        if !result.AllValid {
            allValid = false
        }
    }

    slog.Info("judge data built",
        "problem", problem.ID,
        "version", version,
        "cases", totalCases,
        "all_valid", allValid)

    return &PipelineResult{
        TestCaseCount: totalCases,
        AllValid:      allValid,
        Errors:        allErrors,
    }, nil
}

func (jm *JudgeMaster) GetVersion(problemID string) int {
    jm.mu.RLock()
    defer jm.mu.RUnlock()
    return jm.versions[problemID]
}

func findScripts(dir, pattern string) []string {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil
    }
    var scripts []string
    for _, e := range entries {
        if matched, _ := filepath.Match(pattern, e.Name()); matched {
            scripts = append(scripts, filepath.Join(dir, e.Name()))
        }
    }
    return scripts
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/judgemaster/... -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add internal/judgemaster/
git commit -m "feat: add Judge Master with judge data CI/CD pipeline (generators, validators, reference solutions)"
```

---

### Task 3.2: Judge Data Sync to Judge Servers

**Files:**
- Create: `internal/judgemaster/sync.go`
- Create: `internal/judgemaster/sync_test.go`

- [ ] **Step 1: Write sync test**

```go
// internal/judgemaster/sync_test.go
package judgemaster

import (
    "os"
    "path/filepath"
    "testing"
)

func TestSyncPackage(t *testing.T) {
    tmpdir := t.TempDir()
    problemDir := filepath.Join(tmpdir, "problems", "test-problem")
    os.MkdirAll(problemDir, 0755)

    // Create test data
    os.WriteFile(filepath.Join(problemDir, "in_0001.txt"), []byte("1"), 0644)
    os.WriteFile(filepath.Join(problemDir, "out_0001.txt"), []byte("2"), 0644)

    syncer := NewSyncer(tmpdir)
    pkg, err := syncer.CreatePackage("test-problem", 1)
    if err != nil {
        t.Fatalf("CreatePackage failed: %v", err)
    }

    if pkg.ProblemID != "test-problem" {
        t.Errorf("ProblemID: got %q, want test-problem", pkg.ProblemID)
    }
    if pkg.Version != 1 {
        t.Errorf("Version: got %d, want 1", pkg.Version)
    }
    if len(pkg.Files) < 2 {
        t.Errorf("expected at least 2 files, got %d", len(pkg.Files))
    }
}

func TestSyncDeploy(t *testing.T) {
    tmpdir := t.TempDir()
    problemDir := filepath.Join(tmpdir, "problems", "p1")
    os.MkdirAll(problemDir, 0755)
    os.WriteFile(filepath.Join(problemDir, "in_0001.txt"), []byte("data1"), 0644)

    syncer := NewSyncer(tmpdir)
    pkg, _ := syncer.CreatePackage("p1", 1)

    deployDir := filepath.Join(tmpdir, "deploy", "p1")
    err := syncer.DeployPackage(pkg, deployDir)
    if err != nil {
        t.Fatalf("DeployPackage failed: %v", err)
    }

    // Verify files were deployed
    deployed, err := os.ReadFile(filepath.Join(deployDir, "in_0001.txt"))
    if err != nil {
        t.Fatalf("deployed file not found: %v", err)
    }
    if string(deployed) != "data1" {
        t.Errorf("deployed content: got %q, want data1", string(deployed))
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/judgemaster/ -run TestSync -v`
Expected: FAIL with "undefined: NewSyncer"

- [ ] **Step 3: Implement sync package**

```go
// internal/judgemaster/sync.go
package judgemaster

import (
    "archive/tar"
    "bytes"
    "compress/gzip"
    "fmt"
    "io"
    "os"
    "path/filepath"
)

type Syncer struct {
    dataDir string
}

type DataPackage struct {
    ProblemID string
    Version   int
    Files     map[string][]byte // relative path -> content
}

func NewSyncer(dataDir string) *Syncer {
    return &Syncer{dataDir: dataDir}
}

func (s *Syncer) CreatePackage(problemID string, version int) (*DataPackage, error) {
    problemDir := filepath.Join(s.dataDir, "problems", problemID)
    pkg := &DataPackage{
        ProblemID: problemID,
        Version:   version,
        Files:     make(map[string][]byte),
    }

    entries, err := os.ReadDir(problemDir)
    if err != nil {
        return nil, fmt.Errorf("read problem dir: %w", err)
    }

    for _, entry := range entries {
        if entry.IsDir() {
            continue
        }
        path := filepath.Join(problemDir, entry.Name())
        data, err := os.ReadFile(path)
        if err != nil {
            return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
        }
        pkg.Files[entry.Name()] = data
    }

    return pkg, nil
}

func (s *Syncer) DeployPackage(pkg *DataPackage, targetDir string) error {
    os.MkdirAll(targetDir, 0755)
    for name, data := range pkg.Files {
        path := filepath.Join(targetDir, name)
        if err := os.WriteFile(path, data, 0644); err != nil {
            return fmt.Errorf("write %s: %w", name, err)
        }
    }
    return nil
}

func (s *Syncer) PackageToTarGz(pkg *DataPackage) ([]byte, error) {
    var buf bytes.Buffer
    gz := gzip.NewWriter(&buf)
    tw := tar.NewWriter(gz)

    for name, data := range pkg.Files {
        hdr := &tar.Header{
            Name: name,
            Size: int64(len(data)),
            Mode: 0644,
        }
        if err := tw.WriteHeader(hdr); err != nil {
            return nil, err
        }
        if _, err := tw.Write(data); err != nil {
            return nil, err
        }
    }

    tw.Close()
    gz.Close()
    return buf.Bytes(), nil
}

func (s *Syncer) TarGzToPackage(data []byte) (*DataPackage, error) {
    gr, err := gzip.NewReader(bytes.NewReader(data))
    if err != nil {
        return nil, err
    }
    defer gr.Close()

    tr := tar.NewReader(gr)
    pkg := &DataPackage{Files: make(map[string][]byte)}

    for {
        hdr, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, err
        }
        content, err := io.ReadAll(tr)
        if err != nil {
            return nil, err
        }
        pkg.Files[hdr.Name] = content
    }

    return pkg, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/judgemaster/ -run TestSync -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/judgemaster/sync.go internal/judgemaster/sync_test.go
git commit -m "feat: add judge data sync with tar.gz packaging and deployment"
```

---

## Phase 4: Real-Time Broadcaster (Actor Model)

### Overview
Replace the simple per-submission WebSocket manager with a real-time Broadcaster based on the Actor Model pattern. Uses Go channels for actor communication — no external message broker needed.

### Architecture
```
Load Balancer → TCP → Broadcaster (Go process)
                          ├── Server actor (accepts TCP)
                          ├── ServerHandler actors (one per TCP connection)
                          └── WebSocketHandler actors (one per client connection)

WebSocket Clients ↔ Broadcaster
```

### Files
- Create: `internal/broadcaster/broadcaster.go`
- Create: `internal/broadcaster/server.go`
- Create: `internal/broadcaster/handler.go`
- Create: `internal/broadcaster/broadcaster_test.go`
- Modify: `cmd/aioj/main.go` (start broadcaster goroutine)
- Modify: `internal/api/handler/ws.go` (deprecate old WSManager, connect to broadcaster)
- Modify: `internal/loadbalancer/lb.go` (send status changes to broadcaster)

### Task 4.1: Actor Model Broadcaster

**Files:**
- Create: `internal/broadcaster/broadcaster.go`
- Create: `internal/broadcaster/handler.go`
- Create: `internal/broadcaster/broadcaster_test.go`

- [ ] **Step 1: Write broadcaster test**

```go
// internal/broadcaster/broadcaster_test.go
package broadcaster

import (
    "sync"
    "testing"
    "time"
)

func TestBroadcaster_BroadcastToAll(t *testing.T) {
    b := New()

    var wg sync.WaitGroup
    received := make(map[string][]Message)

    // Create 3 clients
    for i := 0; i < 3; i++ {
        clientID := fmt.Sprintf("client-%d", i)
        ch := make(chan Message, 10)
        b.Subscribe(clientID, ch)
        wg.Add(1)
        go func(cid string, c <-chan Message) {
            defer wg.Done()
            for msg := range c {
                received[cid] = append(received[cid], msg)
            }
        }(clientID, ch)
    }

    // Broadcast a message
    b.Broadcast(Message{Type: "status_update", Data: "sub-1 is now judging"})

    // Give time for delivery
    time.Sleep(50 * time.Millisecond)

    b.Close()

    wg.Wait()

    for cid, msgs := range received {
        if len(msgs) != 1 {
            t.Errorf("%s: expected 1 message, got %d", cid, len(msgs))
        }
    }
}

func TestBroadcaster_Unsubscribe(t *testing.T) {
    b := New()

    ch := make(chan Message, 10)
    b.Subscribe("c1", ch)
    b.Unsubscribe("c1")

    select {
    case <-ch:
        t.Error("channel should be closed after unsubscribe")
    default:
        // expected — channel closed
    }

    b.Close()
}

func TestBroadcaster_NoBlock(t *testing.T) {
    b := New()

    // Create client with very small buffer
    ch := make(chan Message, 1)
    b.Subscribe("slow-client", ch)

    // Fill buffer
    b.Broadcast(Message{Type: "msg1"})
    b.Broadcast(Message{Type: "msg2"})
    b.Broadcast(Message{Type: "msg3"}) // should drop, not block

    time.Sleep(50 * time.Millisecond)
    b.Close()
}

func TestBroadcaster_BroadcastPreservesOrder(t *testing.T) {
    b := New()

    ch := make(chan Message, 100)
    b.Subscribe("c1", ch)

    for i := 0; i < 10; i++ {
        b.Broadcast(Message{Type: "msg", Data: i})
    }

    time.Sleep(50 * time.Millisecond)
    b.Close()

    var received []Message
    for msg := range ch {
        received = append(received, msg)
    }

    if len(received) != 10 {
        t.Fatalf("expected 10 messages, got %d", len(received))
    }
    for i, msg := range received {
        if msg.Data.(int) != i {
            t.Errorf("message %d: expected data %d, got %v", i, i, msg.Data)
        }
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/broadcaster/... -v`
Expected: FAIL — no files in package

- [ ] **Step 3: Implement broadcaster**

```go
// internal/broadcaster/broadcaster.go
package broadcaster

import (
    "sync"
)

type Message struct {
    Type string      `json:"type"`
    Data interface{} `json:"data"`
}

type Broadcaster struct {
    mu      sync.RWMutex
    clients map[string]chan<- Message
    done    chan struct{}
}

func New() *Broadcaster {
    return &Broadcaster{
        clients: make(map[string]chan<- Message),
        done:    make(chan struct{}),
    }
}

func (b *Broadcaster) Subscribe(clientID string, ch chan<- Message) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.clients[clientID] = ch
}

func (b *Broadcaster) Unsubscribe(clientID string) {
    b.mu.Lock()
    defer b.mu.Unlock()
    if ch, ok := b.clients[clientID]; ok {
        close(ch)
        delete(b.clients, clientID)
    }
}

func (b *Broadcaster) Broadcast(msg Message) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    for _, ch := range b.clients {
        select {
        case ch <- msg:
        default:
            // Non-blocking: drop message for slow clients
        }
    }
}

func (b *Broadcaster) BroadcastTo(clientID string, msg Message) bool {
    b.mu.RLock()
    defer b.mu.RUnlock()
    ch, ok := b.clients[clientID]
    if !ok {
        return false
    }
    select {
    case ch <- msg:
        return true
    default:
        return false
    }
}

func (b *Broadcaster) ClientCount() int {
    b.mu.RLock()
    defer b.mu.RUnlock()
    return len(b.clients)
}

func (b *Broadcaster) Close() {
    b.mu.Lock()
    defer b.mu.Unlock()
    for id, ch := range b.clients {
        close(ch)
        delete(b.clients, id)
    }
    close(b.done)
}
```

- [ ] **Step 4: Implement WebSocket handler actor**

```go
// internal/broadcaster/handler.go
package broadcaster

import (
    "log/slog"
    "net/http"
    "net/url"
    "sync"

    "github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool { return true },
}

type WSActor struct {
    mu       sync.Mutex
    conn     *websocket.Conn
    outbox   chan Message
    closed   bool
}

func NewWSActor(conn *websocket.Conn) *WSActor {
    return &WSActor{
        conn:   conn,
        outbox: make(chan Message, 64),
    }
}

func (a *WSActor) Start(b *Broadcaster) {
    clientID := generateClientID()
    ch := make(chan Message, 64)
    b.Subscribe(clientID, ch)

    defer func() {
        b.Unsubscribe(clientID)
        a.Close()
    }()

    // Writer goroutine
    go func() {
        for msg := range ch {
            a.mu.Lock()
            if a.closed {
                a.mu.Unlock()
                return
            }
            err := a.conn.WriteJSON(msg)
            a.mu.Unlock()
            if err != nil {
                slog.Error("ws write error", "client", clientID, "error", err)
                return
            }
        }
    }()

    // Reader goroutine (keep connection alive, handle unsubscribe messages)
    for {
        _, _, err := a.conn.ReadMessage()
        if err != nil {
            break
        }
    }
}

func (a *WSActor) Close() {
    a.mu.Lock()
    defer a.mu.Unlock()
    if !a.closed {
        a.closed = true
        a.conn.Close()
    }
}

func HandleWebSocket(b *Broadcaster) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        conn, err := wsUpgrader.Upgrade(w, r, nil)
        if err != nil {
            slog.Error("ws upgrade failed", "error", err)
            return
        }
        actor := NewWSActor(conn)
        actor.Start(b)
    }
}

var clientIDCounter int
var clientIDMu sync.Mutex

func generateClientID() string {
    clientIDMu.Lock()
    defer clientIDMu.Unlock()
    clientIDCounter++
    return fmt.Sprintf("ws-%d", clientIDCounter)
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/broadcaster/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/broadcaster/
git commit -m "feat: add Actor Model broadcaster with non-blocking WebSocket fan-out"
```

---

### Task 4.2: Integrate Broadcaster into Backend and Load Balancer

**Files:**
- Modify: `cmd/aioj/main.go` (create broadcaster, replace WSManager)
- Modify: `internal/api/handler/submission.go` (use broadcaster for status)
- Modify: `internal/api/router.go` (new WS endpoint)

- [ ] **Step 1: Update main.go to initialize broadcaster**

```go
// In cmd/aioj/main.go:
import "github.com/tahsinarafat/aioj/internal/broadcaster"

// After creating other services:
brd := broadcaster.New()

// Start TCP listener for Load Balancer status updates
go broadcaster.ListenTCP(brd, ":9091")

// Replace wsManager with broadcaster handler:
// Old: r.Get("/ws", wsManager.Handle)
// New: r.Get("/ws", broadcaster.HandleWebSocket(brd))
```

- [ ] **Step 2: Add TCP listener for Load Balancer**

```go
// internal/broadcaster/server.go
package broadcaster

import (
    "bufio"
    "encoding/json"
    "log/slog"
    "net"
)

func ListenTCP(b *Broadcaster, addr string) {
    ln, err := net.Listen("tcp", addr)
    if err != nil {
        slog.Error("broadcaster tcp listen failed", "addr", addr, "error", err)
        return
    }
    defer ln.Close()

    slog.Info("broadcaster tcp listener started", "addr", addr)

    for {
        conn, err := ln.Accept()
        if err != nil {
            slog.Error("broadcaster accept failed", "error", err)
            continue
        }
        go handleTCPConnection(b, conn)
    }
}

func handleTCPConnection(b *Broadcaster, conn net.Conn) {
    defer conn.Close()
    scanner := bufio.NewScanner(conn)
    for scanner.Scan() {
        var msg Message
        if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
            slog.Error("broadcaster invalid tcp message", "error", err)
            continue
        }
        b.Broadcast(msg)
    }
}
```

- [ ] **Step 3: Update LoadBalancer to send status to Broadcaster**

In `internal/loadbalancer/lb.go`, add:

```go
type LoadBalancer struct {
    // ... existing fields
    broadcaster BroadcasterClient
}

type BroadcasterClient interface {
    Send(msg interface{}) error
}

// In Enqueue():
// After successfully enqueuing, notify broadcaster:
if lb.broadcaster != nil {
    lb.broadcaster.Send(map[string]interface{}{
        "type":    "queue_update",
        "server":  serverID,
        "depth":   lb.mq.ServerLen(serverID),
        "total":   lb.totalQueueDepth(),
    })
}
```

- [ ] **Step 4: Wire broadcaster to TCP**

```go
// In loadbalancer/lb.go:
type TCPBroadcaster struct {
    addr string
}

func NewTCPBroadcaster(addr string) *TCPBroadcaster {
    return &TCPBroadcaster{addr: addr}
}

func (t *TCPBroadcaster) Send(msg interface{}) error {
    conn, err := net.Dial("tcp", t.addr)
    if err != nil {
        return err
    }
    defer conn.Close()
    return json.NewEncoder(conn).Encode(msg)
}
```

- [ ] **Step 5: Run build to verify compilation**

Run: `go build ./...`
Expected: No build errors

- [ ] **Step 6: Commit**

```bash
git add internal/broadcaster/server.go cmd/aioj/main.go internal/api/router.go internal/loadbalancer/lb.go
git commit -m "feat: integrate Broadcaster into backend with TCP status channel from Load Balancer"
```

---

## Phase 5: Horizontal Scaling & Network Segmentation

### Overview
Enable running multiple judge-worker instances, with automatic health reporting to the Load Balancer. Add network segmentation via firewall rules so judge servers only talk to the Load Balancer and Judge Master.

### Files
- Modify: `cmd/aioj/main.go` (health heartbeat goroutine for judge-worker mode)
- Modify: `internal/judge/worker.go` (health reporting to LB)
- Modify: `internal/loadbalancer/lb.go` (health check API endpoint)
- Modify: `docker-compose.yml` (multiple judge-worker replicas)
- Create: `deploy/ufw/production-rules.sh`

### Task 5.1: Judge Worker Health Reporting

**Files:**
- Modify: `internal/judge/worker.go`
- Modify: `cmd/aioj/main.go`

- [ ] **Step 1: Add health reporter to WorkerPool**

In `internal/judge/worker.go`, add:

```go
import "github.com/tahsinarafat/aioj/internal/loadbalancer"

type WorkerPool struct {
    // ... existing fields
    serverID     string
    lbClient     *loadbalancer.LBClient
}

// Add after existing NewWorkerPool:
func NewWorkerPoolWithHealth(..., serverID string, lbAddr string) *WorkerPool {
    wp := NewWorkerPool(...)
    wp.serverID = serverID
    wp.lbClient = loadbalancer.NewLBClient(lbAddr)
    return wp
}

// In Start(), add health reporting:
func (wp *WorkerPool) Start(ctx context.Context) {
    // Start health reporter goroutine
    if wp.lbClient != nil {
        go wp.reportHealth(ctx)
    }
    // ... existing code
}

func (wp *WorkerPool) reportHealth(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            activeJobs := len(wp.sem)
            wp.lbClient.ReportHealth(wp.serverID, activeJobs)
        case <-ctx.Done():
            return
        }
    }
}
```

- [ ] **Step 2: Add LB health client**

```go
// internal/loadbalancer/health_client.go
package loadbalancer

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type LBClient struct {
    addr   string
    http   *http.Client
}

func NewLBClient(addr string) *LBClient {
    return &LBClient{
        addr: addr,
        http: &http.Client{Timeout: 5 * time.Second},
    }
}

func (c *LBClient) ReportHealth(serverID string, activeJobs int) error {
    body, _ := json.Marshal(map[string]interface{}{
        "server_id":   serverID,
        "active_jobs": activeJobs,
    })
    _, err := c.http.Post(
        fmt.Sprintf("http://%s/health", c.addr),
        "application/json",
        bytes.NewReader(body),
    )
    return err
}
```

- [ ] **Step 3: Update main.go to use server ID**

```go
// In cmd/aioj/main.go:
if *mode == "judge-worker" {
    serverID := os.Getenv("JUDGE_SERVER_ID")
    if serverID == "" {
        serverID = "judge-1"
    }
    lbAddr := os.Getenv("LB_ADDR")
    if lbAddr == "" {
        lbAddr = "backend:8080"
    }
    // Create WorkerPool with health reporting
    workerPool = judge.NewWorkerPoolWithHealth(..., serverID, lbAddr)
}
```

- [ ] **Step 4: Run build**

Run: `go build ./...`
Expected: No build errors

- [ ] **Step 5: Commit**

```bash
git add internal/judge/worker.go cmd/aioj/main.go internal/loadbalancer/health_client.go
git commit -m "feat: add judge worker health reporting to Load Balancer"
```

---

### Task 5.2: Multiple Judge Workers in Docker Compose

**Files:**
- Modify: `docker-compose.yml`
- Create: `deploy/docker-compose.judge-workers.yml`

- [ ] **Step 1: Add scaled judge workers to docker-compose**

```yaml
# In docker-compose.yml, update judge-worker to support scaling:
  judge-worker:
    build:
      context: .
      dockerfile: Dockerfile
    command: /app/aioj --mode=judge-worker
    environment:
      # ... existing env vars
      JUDGE_SERVER_ID: "judge-{{.Task.Slot}}"
      LB_ADDR: "backend:8080"
    volumes:
      - testdata:/app/testdata
      - ./lang:/app/lang
    depends_on:
      postgres:
        condition: service_healthy
    deploy:
      replicas: 3
    restart: unless-stopped
```

- [ ] **Step 2: Add LB health endpoint to backend API**

```go
// In internal/api/handler/health.go
package handler

import (
    "encoding/json"
    "net/http"
    "github.com/tahsinarafat/aioj/internal/loadbalancer"
)

type HealthHandler struct {
    lb *loadbalancer.LoadBalancer
}

func NewHealthHandler(lb *loadbalancer.LoadBalancer) *HealthHandler {
    return &HealthHandler{lb: lb}
}

func (h *HealthHandler) ReportHealth(w http.ResponseWriter, r *http.Request) {
    var req struct {
        ServerID   string `json:"server_id"`
        ActiveJobs int    `json:"active_jobs"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    if h.lb != nil {
        h.lb.UpdateHealth(req.ServerID, req.ActiveJobs)
    }
    w.WriteHeader(http.StatusOK)
}

func (h *HealthHandler) LBStatus(w http.ResponseWriter, r *http.Request) {
    if h.lb == nil {
        respondJSON(w, http.StatusOK, map[string]interface{}{"status": "not configured"})
        return
    }
    respondJSON(w, http.StatusOK, h.lb.Status())
}
```

- [ ] **Step 3: Register health endpoint in router**

```go
// In internal/api/router.go:
r.Route("/admin", func(r chi.Router) {
    // ... existing routes
    r.Post("/lb/health", healthH.ReportHealth)
    r.Get("/lb/status", healthH.LBStatus)
})
```

- [ ] **Step 4: Add production UFW rules**

```bash
# deploy/ufw/production-rules.sh
#!/bin/bash
set -e

echo "Applying AIOJ production firewall rules..."

# Reset
ufw --force reset

# Default: deny incoming, allow outgoing
ufw default deny incoming
ufw default allow outgoing

# SSH
ufw allow 22/tcp

# HTTP/HTTPS (Caddy/Nginx)
ufw allow 80/tcp
ufw allow 443/tcp

# Backend API (only from internal network — adjust subnet)
ufw allow from 10.0.0.0/8 to any port 8080 proto tcp

# Judge servers talk only to backend
ufw allow from 10.0.1.0/24 to any port 5050 proto tcp  # go-judge
ufw allow from 10.0.1.0/24 to any port 8080 proto tcp  # backend health

# Enable
ufw --force enable
ufw status verbose
```

- [ ] **Step 5: Commit**

```bash
git add docker-compose.yml deploy/docker-compose.judge-workers.yml deploy/ufw/production-rules.sh internal/api/handler/health.go internal/api/router.go
git commit -m "feat: add horizontal scaling with multiple judge workers and health reporting"
```

---

## Phase 6: Advanced Features & Infrastructure Hardening

### Task 6.1: Rate Limiting Middleware

**Files:**
- Create: `internal/api/middleware/ratelimit.go`
- Create: `internal/api/middleware/ratelimit_test.go`

### Task 6.2: Submission Deduplication

**Files:**
- Modify: `internal/api/handler/submission.go`

### Task 6.3: Judge Data Version Tracking in DB

**Files:**
- Modify: `internal/store/postgres/problems.go`
- Create: `internal/store/migrations/` (new migration for judge_data_versions table)

### Task 6.4: Anti-Cheating Measures (submit rate cap per user/problem)

**Files:**
- Create: `internal/anticheat/rate_limiter.go`
- Modify: `internal/api/handler/submission.go`

---

## Summary of All New/Modified Files

### New Packages
| Package | Purpose |
|---------|---------|
| `internal/loadbalancer/` | Load balancer with multi-queue scheduling |
| `internal/judgemaster/` | Judge Master CI/CD pipeline |
| `internal/broadcaster/` | Actor Model real-time broadcaster |
| `internal/anticheat/` | Anti-cheating rate limits |

### New Endpoints
| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/submissions/{id}/rejudge` | Re-judge a submission |
| POST | `/api/admin/lb/health` | Judge server health heartbeat |
| GET | `/api/admin/lb/status` | Load balancer status dashboard |
| GET | `/ws` | Real-time WebSocket (broadcaster-backed) |

### Configuration Additions
| Config Key | Purpose |
|------------|---------|
| `load_balancer.max_queue_depth` | Max pending per judge server |
| `load_balancer.health_ttl` | Judge health timeout |
| `load_balancer.scheduler` | Scheduling algorithm |
| `JUDGE_SERVERS` (env) | Comma-separated judge server IDs |
| `JUDGE_SERVER_ID` (env) | This judge worker's ID |
| `LB_ADDR` (env) | Load balancer health endpoint |

---

> **Plan complete.** This is a massive undertaking across 6 phases. Each phase produces working, testable software independently. Start with Phase 1 for immediate value (verdicts, SPJ caching, re-judging), then scale progressively through Phases 2-6.

