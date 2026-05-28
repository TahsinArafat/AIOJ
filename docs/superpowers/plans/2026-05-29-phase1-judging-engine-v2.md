# Phase 1: Judging Engine v2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform AIOJ's judging engine from basic standard I/O into a world-class system supporting interactive problems, submit-answer (output-only) problems, IOI-style subtask scoring, 8 checker types, and per-language resource limits.

**Architecture:** Extend the existing Go monolith's judge worker (`internal/judge/worker.go`) with new execution paths for interactive and submit-answer problems. Add subtask scoring by introducing `subtask_id` and `subtask_score` columns to test cases. Expand the checker registry with 5 new types. Add a `language_limits` table for per-problem per-language resource overrides.

**Tech Stack:** Go 1.21+, PostgreSQL 18, criyle/go-judge sandbox, React 19 + TypeScript + Tailwind CSS

---

## File Structure

### Files to Create
```
internal/judge/checker/float_absolute.go    — FloatAbsolute checker
internal/judge/checker/float_relative.go    — FloatRelative checker  
internal/judge/checker/sorted.go            — Sorted checker
internal/judge/checker/unordered.go         — Unordered (multiset) checker
internal/judge/checker/byte_identical.go    — ByteIdentical checker
internal/judge/interactive.go               — Interactive judging logic (pipe interactor ↔ contestant)
internal/model/language_limit.go            — LanguageLimit model
internal/store/postgres/language_limits.go  — LanguageLimit store
internal/api/handler/import.go              — FPS import handler (Phase 2, not here)
```

### Files to Modify
```
internal/model/problem.go                   — Add ProblemType, SubtaskID, SubtaskScore fields
internal/model/submission.go                — Add SubmissionType field
internal/model/model_test.go                — Add tests for new types
internal/judge/worker.go                    — Add interactive + submit-answer + subtask paths
internal/judge/checker/checker.go           — Register 5 new checkers in GetChecker()
internal/store/interfaces.go                — Add LanguageLimitStore interface
internal/store/postgres/problems.go         — Update queries for subtask fields
internal/store/postgres/submissions.go      — Update queries for submission_type
internal/api/handler/submission.go          — Accept file uploads for submit-answer
internal/api/router.go                      — Register new routes
internal/store/migrations/000023_subtask_scoring.up.sql — Subtask columns
internal/store/migrations/000024_submission_type.up.sql — Submission type column
internal/store/migrations/000025_language_limits.up.sql — Language limits table
cmd/aioj/main.go                            — Wire LanguageLimitStore
web/src/pages/ProblemDetail.tsx              — UI for interactive/submit-answer indicators
web/src/pages/SubmissionDetail.tsx           — Show subtask scores
```

---

## Task 1: Database Migrations

**Files:**
- Create: `internal/store/migrations/000023_subtask_scoring.up.sql`
- Create: `internal/store/migrations/000023_subtask_scoring.down.sql`
- Create: `internal/store/migrations/000024_submission_type.up.sql`
- Create: `internal/store/migrations/000024_submission_type.down.sql`
- Create: `internal/store/migrations/000025_language_limits.up.sql`
- Create: `internal/store/migrations/000025_language_limits.down.sql`

- [ ] **Step 1: Create subtask scoring migration**

```sql
-- 000023_subtask_scoring.up.sql
-- Add subtask support to test cases and problems

-- Add subtask columns to existing test case JSONB (stored in testcase_score)
-- No schema change needed — subtask_id and subtask_score go into the JSONB array elements
-- But we need a problem-level config:
ALTER TABLE problems 
    ADD COLUMN scoring_mode VARCHAR(16) NOT NULL DEFAULT 'complete',
    ADD COLUMN subtask_aggregation VARCHAR(8) NOT NULL DEFAULT 'min';

-- Add constraint
ALTER TABLE problems 
    ADD CONSTRAINT problems_scoring_mode_check 
    CHECK (scoring_mode IN ('complete', 'partial'));

ALTER TABLE problems 
    ADD CONSTRAINT problems_subtask_aggregation_check 
    CHECK (subtask_aggregation IN ('min', 'sum'));
```

- [ ] **Step 2: Create down migration**

```sql
-- 000023_subtask_scoring.down.sql
ALTER TABLE problems DROP COLUMN IF EXISTS scoring_mode;
ALTER TABLE problems DROP COLUMN IF EXISTS subtask_aggregation;
```

- [ ] **Step 3: Create submission type migration**

```sql
-- 000024_submission_type.up.sql
-- Add submission type (code vs output-only)
ALTER TABLE submissions 
    ADD COLUMN submission_type VARCHAR(16) NOT NULL DEFAULT 'code';

ALTER TABLE submissions 
    ADD CONSTRAINT submissions_type_check 
    CHECK (submission_type IN ('code', 'output'));
```

- [ ] **Step 4: Create down migration**

```sql
-- 000024_submission_type.down.sql
ALTER TABLE submissions DROP COLUMN IF EXISTS submission_type;
```

- [ ] **Step 5: Create language limits migration**

```sql
-- 000025_language_limits.up.sql
-- Per-problem per-language resource limits
CREATE TABLE IF NOT EXISTS language_limits (
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    language_id VARCHAR(64) NOT NULL,
    time_limit_ms INTEGER,
    memory_limit_kb INTEGER,
    PRIMARY KEY (problem_id, language_id)
);

CREATE INDEX idx_language_limits_problem ON language_limits(problem_id);
```

- [ ] **Step 6: Create down migration**

```sql
-- 000025_language_limits.down.sql
DROP TABLE IF EXISTS language_limits;
```

- [ ] **Step 7: Verify migrations run**

Run: `make migrate-up`
Expected: All 3 new migrations apply cleanly (000023, 000024, 000025)

- [ ] **Step 8: Commit**

```bash
git add internal/store/migrations/000023_* internal/store/migrations/000024_* internal/store/migrations/000025_*
git commit -m "feat: add migrations for subtask scoring, submission type, and language limits"
```

---

## Task 2: Model Updates

**Files:**
- Modify: `internal/model/problem.go`
- Modify: `internal/model/submission.go`
- Create: `internal/model/language_limit.go`
- Modify: `internal/model/model_test.go`

- [ ] **Step 1: Update Problem model with scoring fields**

Add to `internal/model/problem.go` after the existing `Problem` struct fields:

```go
// Add these fields to the Problem struct (after InteractorSourceCode):
    ScoringMode         string `json:"scoring_mode"`          // "complete" or "partial"
    SubtaskAggregation  string `json:"subtask_aggregation"`   // "min" or "sum"
```

Add a new method to `Problem`:

```go
// GetSubtasks extracts subtask groupings from TestCaseScore.
// Returns a map of subtask_id → []TestCaseScore.
func (p *Problem) GetSubtasks() map[int][]TestCaseScore {
    subtasks := make(map[int][]TestCaseScore)
    for _, tc := range p.TestCaseScore {
        if tc.SubtaskID > 0 {
            subtasks[tc.SubtaskID] = append(subtasks[tc.SubtaskID], tc)
        }
    }
    return subtasks
}

// HasSubtasks returns true if any test case has a subtask_id > 0.
func (p *Problem) HasSubtasks() bool {
    for _, tc := range p.TestCaseScore {
        if tc.SubtaskID > 0 {
            return true
        }
    }
    return false
}
```

- [ ] **Step 2: Update TestCaseScore with subtask fields**

In `internal/model/problem.go`, update the `TestCaseScore` struct:

```go
type TestCaseScore struct {
    InputName   string `json:"input_name"`
    OutputName  string `json:"output_name"`
    Score       int    `json:"score"`
    SubtaskID   int    `json:"subtask_id,omitempty"`   // NEW: 0 = no subtask
    SubtaskScore int   `json:"subtask_score,omitempty"` // NEW: score within subtask
}
```

- [ ] **Step 3: Update Submission model with type field**

In `internal/model/submission.go`, add to the `Submission` struct:

```go
// Add after the existing fields:
    SubmissionType string `json:"submission_type"` // "code" or "output"
```

Add a constant:

```go
const (
    SubmissionTypeCode   SubmissionType = "code"
    SubmissionTypeOutput SubmissionType = "output"
)

type SubmissionType string
```

Update `SubmitRequest` to include optional type:

```go
type SubmitRequest struct {
    ProblemID    string `json:"problem_id"`
    Language     string `json:"language"`
    SourceCode   string `json:"source_code"`
    ContestID    string `json:"contest_id,omitempty"`
    SubmissionType string `json:"submission_type,omitempty"` // "code" or "output", default "code"
}
```

- [ ] **Step 4: Create LanguageLimit model**

Create `internal/model/language_limit.go`:

```go
package model

// LanguageLimit defines per-language resource overrides for a problem.
type LanguageLimit struct {
    ProblemID    string `json:"problem_id"`
    LanguageID   string `json:"language_id"`
    TimeLimitMs  *int   `json:"time_limit_ms,omitempty"`  // nil = use problem default
    MemoryLimitKB *int  `json:"memory_limit_kb,omitempty"` // nil = use problem default
}
```

- [ ] **Step 5: Add tests for new model functionality**

Add to `internal/model/model_test.go`:

```go
func TestProblemHasSubtasks(t *testing.T) {
    p := &Problem{
        TestCaseScore: []TestCaseScore{
            {InputName: "1.in", OutputName: "1.out", Score: 10, SubtaskID: 1},
            {InputName: "2.in", OutputName: "2.out", Score: 10, SubtaskID: 1},
            {InputName: "3.in", OutputName: "3.out", Score: 20, SubtaskID: 2},
        },
    }
    
    if !p.HasSubtasks() {
        t.Error("expected HasSubtasks() = true")
    }
    
    subtasks := p.GetSubtasks()
    if len(subtasks) != 2 {
        t.Errorf("expected 2 subtasks, got %d", len(subtasks))
    }
    if len(subtasks[1]) != 2 {
        t.Errorf("expected 2 cases in subtask 1, got %d", len(subtasks[1]))
    }
}

func TestProblemNoSubtasks(t *testing.T) {
    p := &Problem{
        TestCaseScore: []TestCaseScore{
            {InputName: "1.in", OutputName: "1.out", Score: 10},
            {InputName: "2.in", OutputName: "2.out", Score: 10},
        },
    }
    
    if p.HasSubtasks() {
        t.Error("expected HasSubtasks() = false")
    }
}
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/model/ -v -run TestProblem`
Expected: All tests pass

- [ ] **Step 7: Commit**

```bash
git add internal/model/problem.go internal/model/submission.go internal/model/language_limit.go internal/model/model_test.go
git commit -m "feat: add scoring_mode, subtask fields, submission_type, and LanguageLimit model"
```

---

## Task 3: New Checker Types

**Files:**
- Create: `internal/judge/checker/float_absolute.go`
- Create: `internal/judge/checker/float_relative.go`
- Create: `internal/judge/checker/sorted.go`
- Create: `internal/judge/checker/unordered.go`
- Create: `internal/judge/checker/byte_identical.go`
- Modify: `internal/judge/checker/checker.go`
- Modify: `internal/judge/checker/checker_test.go` (create if doesn't exist)

- [ ] **Step 1: Create FloatAbsolute checker**

Create `internal/judge/checker/float_absolute.go`:

```go
package checker

import (
    "math"
    "strconv"
    "strings"
)

// FloatAbsoluteChecker compares floating-point outputs with absolute tolerance.
// |expected - actual| <= epsilon
type FloatAbsoluteChecker struct {
    Epsilon float64
}

func NewFloatAbsoluteChecker(epsilon float64) *FloatAbsoluteChecker {
    if epsilon <= 0 {
        epsilon = 1e-6
    }
    return &FloatAbsoluteChecker{Epsilon: epsilon}
}

func (c *FloatAbsoluteChecker) Name() string { return "float_absolute" }

func (c *FloatAbsoluteChecker) Check(input, expected, actual []byte) *Result {
    expectedTokens := tokenizeFloats(string(expected))
    actualTokens := tokenizeFloats(string(actual))
    
    if len(expectedTokens) != len(actualTokens) {
        return &Result{Passed: false, Score: 0, Message: "token count mismatch"}
    }
    
    for i := range expectedTokens {
        e, err1 := strconv.ParseFloat(expectedTokens[i], 64)
        a, err2 := strconv.ParseFloat(actualTokens[i], 64)
        if err1 != nil || err2 != nil {
            if expectedTokens[i] != actualTokens[i] {
                return &Result{Passed: false, Score: 0, Message: "non-numeric token mismatch"}
            }
            continue
        }
        if math.Abs(e-a) > c.Epsilon {
            return &Result{Passed: false, Score: 0, Message: "value mismatch"}
        }
    }
    
    return &Result{Passed: true, Score: 100, Message: "OK"}
}

func tokenizeFloats(s string) []string {
    return strings.Fields(s)
}
```

- [ ] **Step 2: Create FloatRelative checker**

Create `internal/judge/checker/float_relative.go`:

```go
package checker

import (
    "math"
    "strconv"
)

// FloatRelativeChecker compares floating-point outputs with relative tolerance.
// |expected - actual| / max(|expected|, 1) <= epsilon
type FloatRelativeChecker struct {
    Epsilon float64
}

func NewFloatRelativeChecker(epsilon float64) *FloatRelativeChecker {
    if epsilon <= 0 {
        epsilon = 1e-6
    }
    return &FloatRelativeChecker{Epsilon: epsilon}
}

func (c *FloatRelativeChecker) Name() string { return "float_relative" }

func (c *FloatRelativeChecker) Check(input, expected, actual []byte) *Result {
    expectedTokens := tokenizeFloats(string(expected))
    actualTokens := tokenizeFloats(string(actual))
    
    if len(expectedTokens) != len(actualTokens) {
        return &Result{Passed: false, Score: 0, Message: "token count mismatch"}
    }
    
    for i := range expectedTokens {
        e, err1 := strconv.ParseFloat(expectedTokens[i], 64)
        a, err2 := strconv.ParseFloat(actualTokens[i], 64)
        if err1 != nil || err2 != nil {
            if expectedTokens[i] != actualTokens[i] {
                return &Result{Passed: false, Score: 0, Message: "non-numeric token mismatch"}
            }
            continue
        }
        denom := math.Max(math.Abs(e), 1)
        if math.Abs(e-a)/denom > c.Epsilon {
            return &Result{Passed: false, Score: 0, Message: "relative mismatch"}
        }
    }
    
    return &Result{Passed: true, Score: 100, Message: "OK"}
}
```

- [ ] **Step 3: Create Sorted checker**

Create `internal/judge/checker/sorted.go`:

```go
package checker

import (
    "sort"
    "strings"
)

// SortedChecker sorts both outputs line-by-line and compares.
type SortedChecker struct{}

func (c *SortedChecker) Name() string { return "sorted" }

func (c *SortedChecker) Check(input, expected, actual []byte) *Result {
    expectedLines := sortedLines(string(expected))
    actualLines := sortedLines(string(actual))
    
    if len(expectedLines) != len(actualLines) {
        return &Result{Passed: false, Score: 0, Message: "line count mismatch"}
    }
    
    for i := range expectedLines {
        if expectedLines[i] != actualLines[i] {
            return &Result{Passed: false, Score: 0, Message: "line mismatch after sorting"}
        }
    }
    
    return &Result{Passed: true, Score: 100, Message: "OK"}
}

func sortedLines(s string) []string {
    lines := strings.Split(strings.TrimSpace(s), "\n")
    sort.Strings(lines)
    return lines
}
```

- [ ] **Step 4: Create Unordered (multiset) checker**

Create `internal/judge/checker/unordered.go`:

```go
package checker

import (
    "sort"
    "strings"
)

// UnorderedChecker compares outputs as multisets (order-independent, with duplicates).
type UnorderedChecker struct{}

func (c *UnorderedChecker) Name() string { return "unordered" }

func (c *UnorderedChecker) Check(input, expected, actual []byte) *Result {
    expectedTokens := sortedTokens(string(expected))
    actualTokens := sortedTokens(string(actual))
    
    if len(expectedTokens) != len(actualTokens) {
        return &Result{Passed: false, Score: 0, Message: "token count mismatch"}
    }
    
    for i := range expectedTokens {
        if expectedTokens[i] != actualTokens[i] {
            return &Result{Passed: false, Score: 0, Message: "token mismatch after sorting"}
        }
    }
    
    return &Result{Passed: true, Score: 100, Message: "OK"}
}

func sortedTokens(s string) []string {
    tokens := strings.Fields(s)
    sort.Strings(tokens)
    return tokens
}
```

- [ ] **Step 5: Create ByteIdentical checker**

Create `internal/judge/checker/byte_identical.go`:

```go
package checker

import "bytes"

// ByteIdenticalChecker does exact byte comparison with no whitespace trimming.
type ByteIdenticalChecker struct{}

func (c *ByteIdenticalChecker) Name() string { return "byte_identical" }

func (c *ByteIdenticalChecker) Check(input, expected, actual []byte) *Result {
    if bytes.Equal(expected, actual) {
        return &Result{Passed: true, Score: 100, Message: "OK"}
    }
    return &Result{Passed: false, Score: 0, Message: "byte mismatch"}
}
```

- [ ] **Step 6: Register all checkers in GetChecker()**

Update `internal/judge/checker/checker.go` — modify the `GetChecker` function:

```go
func GetChecker(name string, epsilon float64) Checker {
    switch name {
    case "lines":
        return &LinesChecker{}
    case "float":
        return &FloatChecker{Epsilon: epsilon}
    case "float_absolute":
        return NewFloatAbsoluteChecker(epsilon)
    case "float_relative":
        return NewFloatRelativeChecker(epsilon)
    case "sorted":
        return &SortedChecker{}
    case "unordered":
        return &UnorderedChecker{}
    case "byte_identical":
        return &ByteIdenticalChecker{}
    default:
        return &ExactChecker{}
    }
}
```

Note: Update the signature to accept `epsilon float64` (currently `GetChecker` only takes `name string`). Update all callers.

- [ ] **Step 7: Write checker tests**

Create `internal/judge/checker/checker_test.go`:

```go
package checker

import (
    "testing"
)

func TestFloatAbsoluteChecker(t *testing.T) {
    c := NewFloatAbsoluteChecker(1e-6)
    
    // Exact match
    r := c.Check(nil, []byte("3.14159"), []byte("3.14159"))
    if !r.Passed { t.Error("expected pass") }
    
    // Within tolerance
    r = c.Check(nil, []byte("3.14159"), []byte("3.14160"))
    if !r.Passed { t.Error("expected pass within tolerance") }
    
    // Outside tolerance
    r = c.Check(nil, []byte("3.14159"), []byte("3.15"))
    if r.Passed { t.Error("expected fail outside tolerance") }
}

func TestFloatRelativeChecker(t *testing.T) {
    c := NewFloatRelativeChecker(0.01) // 1% tolerance
    
    // 0.1 vs 0.1001 → ~0.1% relative error → pass
    r := c.Check(nil, []byte("0.1"), []byte("0.1001"))
    if !r.Passed { t.Error("expected pass") }
    
    // 0.1 vs 0.11 → 10% relative error → fail
    r = c.Check(nil, []byte("0.1"), []byte("0.11"))
    if r.Passed { t.Error("expected fail") }
}

func TestSortedChecker(t *testing.T) {
    c := &SortedChecker{}
    
    r := c.Check(nil, []byte("3\n1\n2"), []byte("1\n2\n3"))
    if !r.Passed { t.Error("expected pass (same lines, different order)") }
    
    r = c.Check(nil, []byte("3\n1\n2"), []byte("1\n2\n4"))
    if r.Passed { t.Error("expected fail (different content)") }
}

func TestUnorderedChecker(t *testing.T) {
    c := &UnorderedChecker{}
    
    r := c.Check(nil, []byte("a b c"), []byte("c a b"))
    if !r.Passed { t.Error("expected pass (same tokens)") }
    
    r = c.Check(nil, []byte("a a b"), []byte("a b b"))
    if r.Passed { t.Error("expected fail (different multisets)") }
}

func TestByteIdenticalChecker(t *testing.T) {
    c := &ByteIdenticalChecker{}
    
    r := c.Check(nil, []byte("hello"), []byte("hello"))
    if !r.Passed { t.Error("expected pass") }
    
    r = c.Check(nil, []byte("hello"), []byte("hello\n"))
    if r.Passed { t.Error("expected fail (trailing newline)") }
}
```

- [ ] **Step 8: Run all checker tests**

Run: `go test ./internal/judge/checker/ -v`
Expected: All tests pass

- [ ] **Step 9: Commit**

```bash
git add internal/judge/checker/
git commit -m "feat: add 5 new checker types (float_absolute, float_relative, sorted, unordered, byte_identical)"
```

---

## Task 4: Store Layer Updates

**Files:**
- Modify: `internal/store/interfaces.go`
- Modify: `internal/store/postgres/problems.go`
- Modify: `internal/store/postgres/submissions.go`
- Create: `internal/store/postgres/language_limits.go`

- [ ] **Step 1: Add LanguageLimitStore interface**

Add to `internal/store/interfaces.go`:

```go
// LanguageLimitStore manages per-problem per-language resource limits.
type LanguageLimitStore interface {
    Set(ctx context.Context, limit *model.LanguageLimit) error
    Get(ctx context.Context, problemID, languageID string) (*model.LanguageLimit, error)
    GetByProblem(ctx context.Context, problemID string) ([]*model.LanguageLimit, error)
    Delete(ctx context.Context, problemID, languageID string) error
}
```

- [ ] **Step 2: Implement LanguageLimitStore**

Create `internal/store/postgres/language_limits.go`:

```go
package postgres

import (
    "context"
    "database/sql"
    
    "github.com/AIOJ/AIOJ/internal/model"
)

type LanguageLimitStore struct {
    db *sql.DB
}

func NewLanguageLimitStore(db *sql.DB) *LanguageLimitStore {
    return &LanguageLimitStore{db: db}
}

func (s *LanguageLimitStore) Set(ctx context.Context, limit *model.LanguageLimit) error {
    _, err := s.db.ExecContext(ctx, `
        INSERT INTO language_limits (problem_id, language_id, time_limit_ms, memory_limit_kb)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (problem_id, language_id) 
        DO UPDATE SET time_limit_ms = $3, memory_limit_kb = $4`,
        limit.ProblemID, limit.LanguageID, limit.TimeLimitMs, limit.MemoryLimitKB)
    return err
}

func (s *LanguageLimitStore) Get(ctx context.Context, problemID, languageID string) (*model.LanguageLimit, error) {
    var l model.LanguageLimit
    err := s.db.QueryRowContext(ctx, `
        SELECT problem_id, language_id, time_limit_ms, memory_limit_kb
        FROM language_limits WHERE problem_id = $1 AND language_id = $2`,
        problemID, languageID).Scan(&l.ProblemID, &l.LanguageID, &l.TimeLimitMs, &l.MemoryLimitKB)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    return &l, err
}

func (s *LanguageLimitStore) GetByProblem(ctx context.Context, problemID string) ([]*model.LanguageLimit, error) {
    rows, err := s.db.QueryContext(ctx, `
        SELECT problem_id, language_id, time_limit_ms, memory_limit_kb
        FROM language_limits WHERE problem_id = $1`,
        problemID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var limits []*model.LanguageLimit
    for rows.Next() {
        var l model.LanguageLimit
        if err := rows.Scan(&l.ProblemID, &l.LanguageID, &l.TimeLimitMs, &l.MemoryLimitKB); err != nil {
            return nil, err
        }
        limits = append(limits, &l)
    }
    return limits, nil
}

func (s *LanguageLimitStore) Delete(ctx context.Context, problemID, languageID string) error {
    _, err := s.db.ExecContext(ctx, `
        DELETE FROM language_limits WHERE problem_id = $1 AND language_id = $2`,
        problemID, languageID)
    return err
}
```

- [ ] **Step 3: Update ProblemStore queries for new fields**

In `internal/store/postgres/problems.go`, update the `Create` method's INSERT to include `scoring_mode` and `subtask_aggregation`. Update the `getBy` method's SELECT to include these fields. Update `Update` to handle them.

Also update `CreateProblemRequest` in `internal/model/problem.go` to include:

```go
ScoringMode        string `json:"scoring_mode,omitempty"`        // default "complete"
SubtaskAggregation string `json:"subtask_aggregation,omitempty"` // default "min"
```

- [ ] **Step 4: Update SubmissionStore queries for submission_type**

In `internal/store/postgres/submissions.go`, update `Create` to include `submission_type` in INSERT. Update `GetByID` to SELECT `submission_type`.

- [ ] **Step 5: Run store tests**

Run: `go test ./internal/store/... -v`
Expected: All tests pass (may need to update existing tests for new fields)

- [ ] **Step 6: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/language_limits.go internal/store/postgres/problems.go internal/store/postgres/submissions.go
git commit -m "feat: add LanguageLimitStore and update store queries for subtask/scoring fields"
```

---

## Task 5: Interactive Judging Implementation

**Files:**
- Create: `internal/judge/interactive.go`
- Modify: `internal/judge/worker.go`

- [ ] **Step 1: Create interactive judging module**

Create `internal/judge/interactive.go`:

```go
package judge

import (
    "context"
    "fmt"
    
    "github.com/AIOJ/AIOJ/internal/judge/executor"
)

// InteractiveResult holds the outcome of an interactive judging session.
type InteractiveResult struct {
    Status     string
    Time       uint64
    Memory     uint64
    Message    string
}

// runInteractive executes an interactor and contestant program connected via pipes.
// The interactor's stdout → contestant's stdin, contestant's stdout → interactor's stdin.
func (wp *WorkerPool) runInteractive(ctx context.Context, req executor.ExecRequest, interactorCmd []string, contestantCmd []string) (*InteractiveResult, error) {
    // Build two commands connected via PipeInput
    // go-judge supports PipeInput: connects cmd[0].stdout → cmd[1].stdin
    
    interactorExec := executor.Cmd{
        Args:        interactorCmd,
        CPULimit:    30_000_000_000, // 30s for interactor
        MemoryLimit: 512 * 1024 * 1024,
        ProcLimit:   64,
    }
    
    contestantExec := executor.Cmd{
        Args:        contestantCmd,
        CPULimit:    req.Cmd[0].CPULimit,
        MemoryLimit: req.Cmd[0].MemoryLimit,
        ProcLimit:   req.Cmd[0].ProcLimit,
        CopyIn:      req.Cmd[0].CopyIn,
        CopyOut:     req.Cmd[0].CopyOut,
    }
    
    interactiveReq := executor.ExecRequest{
        Cmd:       []executor.Cmd{interactorExec, contestantExec},
        PipeInput: true,
    }
    
    results, err := wp.exec.Run(interactiveReq)
    if err != nil {
        return nil, fmt.Errorf("interactive execution failed: %w", err)
    }
    
    if len(results) < 2 {
        return nil, fmt.Errorf("expected 2 results, got %d", len(results))
    }
    
    // Interactor result determines the verdict
    interactorResult := results[0]
    contestantResult := results[1]
    
    // Check for contestant crashes
    if contestantResult.Status == "RuntimeError" {
        return &InteractiveResult{
            Status:  "RE",
            Time:    contestantResult.Time,
            Memory:  contestantResult.Memory,
            Message: "Runtime error in contestant program",
        }, nil
    }
    
    // Check for time/memory limits
    if contestantResult.Status == "TimeLimitExceeded" {
        return &InteractiveResult{
            Status:  "TLE",
            Time:    contestantResult.Time,
            Memory:  contestantResult.Memory,
        }, nil
    }
    if contestantResult.Status == "MemoryLimitExceeded" {
        return &InteractiveResult{
            Status:  "MLE",
            Time:    contestantResult.Time,
            Memory:  contestantResult.Memory,
        }, nil
    }
    
    // Interactor exit code determines AC/WA
    // Convention: exit 0 = AC, exit 1 = WA, exit 2+ = FAIL (IE)
    switch interactorResult.ExitStatus {
    case 0:
        return &InteractiveResult{
            Status:  "ac",
            Time:    contestantResult.Time,
            Memory:  contestantResult.Memory,
            Message: interactorResult.Error,
        }, nil
    case 1:
        return &InteractiveResult{
            Status:  "wa",
            Time:    contestantResult.Time,
            Memory:  contestantResult.Memory,
            Message: interactorResult.Error,
        }, nil
    default:
        return &InteractiveResult{
            Status:  "se",
            Time:    contestantResult.Time,
            Memory:  contestantResult.Memory,
            Message: fmt.Sprintf("interactor exited with code %d", interactorResult.ExitStatus),
        }, nil
    }
}
```

- [ ] **Step 2: Update worker.go to handle interactive problems**

In `internal/judge/worker.go`, replace the existing interactive check (around line 75-81) that returns SE:

```go
// OLD:
if prob.Interactive {
    // Interactive problems not yet supported
    wp.subStore.UpdateResult(ctx, sub.ID, model.StatusSE, 0, 0, 0, "Interactive problems not yet supported", nil)
    return
}

// NEW:
if prob.Interactive {
    wp.judgeInteractive(ctx, sub, prob, langCfg)
    return
}
```

Add the new method:

```go
func (wp *WorkerPool) judgeInteractive(ctx context.Context, sub *model.Submission, prob *model.Problem, langCfg LanguageConfig) {
    // 1. Compile contestant code
    compileReq := buildCompileRequest(sub, prob, langCfg)
    compileResult, err := wp.exec.Run(compileReq)
    if err != nil || compileResult[0].Status != "Accepted" {
        compileOutput := ""
        if len(compileResult) > 0 {
            compileOutput = compileResult[0].Error
        }
        wp.subStore.UpdateResult(ctx, sub.ID, model.StatusCE, 0, 0, 0, compileOutput, nil)
        return
    }
    
    // 2. Compile interactor
    interactorLang := wp.loadLangConfig(prob.InteractorLanguage)
    interactorExe := compileInteractor(prob, interactorLang)
    
    // 3. Run interactive session
    contestantCmd := buildRunCommand(sub, prob, langCfg)
    result, err := wp.runInteractive(ctx, /* ... */, interactorExe, contestantCmd)
    if err != nil {
        wp.subStore.UpdateResult(ctx, sub.ID, model.StatusSE, 0, 0, 0, err.Error(), nil)
        return
    }
    
    // 4. Map status
    status := model.SubmissionStatus(result.Status)
    wp.subStore.UpdateResult(ctx, sub.ID, status, 0, result.Time, result.Memory, result.Message, nil)
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/judge/... -v`
Expected: Tests pass (existing tests unaffected, new interactive code compiles)

- [ ] **Step 4: Commit**

```bash
git add internal/judge/interactive.go internal/judge/worker.go
git commit -m "feat: implement interactive problem judging with pipe-based interactor protocol"
```

---

## Task 6: Submit-Answer (Output-Only) Judging

**Files:**
- Modify: `internal/judge/worker.go`
- Modify: `internal/api/handler/submission.go`

- [ ] **Step 1: Add submit-answer path in worker.go**

In `internal/judge/worker.go`, add a check near the top of the `judge()` function (after loading submission and problem):

```go
// Submit-answer (output-only) problems: skip compilation, compare uploaded output directly
if sub.SubmissionType == "output" {
    wp.judgeOutputOnly(ctx, sub, prob)
    return
}
```

Add the new method:

```go
func (wp *WorkerPool) judgeOutputOnly(ctx context.Context, sub *model.Submission, prob *model.Problem) {
    // The source_code field contains the contestant's uploaded output
    contestantOutput := []byte(sub.SourceCode)
    
    var results []model.TestCaseResult
    totalScore := 0
    maxScore := 0
    allPassed := true
    
    for _, tc := range prob.TestCaseScore {
        maxScore += tc.Score
        
        // Load expected output
        expectedOutput, err := wp.loadFile(fmt.Sprintf("%s/%s", prob.TestdataPath, tc.OutputName))
        if err != nil {
            results = append(results, model.TestCaseResult{
                CaseName: tc.InputName,
                Status:   "se",
                Score:    0,
                Detail:   fmt.Sprintf("failed to load expected output: %v", err),
            })
            allPassed = false
            continue
        }
        
        // Get checker
        checker := getChecker(prob.CheckerType, prob.FloatEpsilon)
        result := checker.Check(nil, expectedOutput, contestantOutput)
        
        score := 0
        status := "wa"
        if result.Passed {
            score = tc.Score
            status = "ac"
        } else {
            allPassed = false
        }
        totalScore += score
        
        results = append(results, model.TestCaseResult{
            CaseName: tc.InputName,
            Status:   status,
            Score:    score,
            Detail:   result.Message,
        })
    }
    
    finalStatus := model.StatusWA
    if allPassed {
        finalStatus = model.StatusAC
    }
    
    // Calculate percentage score for OI mode
    percentageScore := 0
    if maxScore > 0 {
        percentageScore = (totalScore * 100) / maxScore
    }
    
    wp.subStore.UpdateResult(ctx, sub.ID, finalStatus, percentageScore, 0, 0, "", results)
    wp.probStore.UpdateCounts(ctx, sub.ProblemID, true, boolToInt(finalStatus == model.StatusAC))
}
```

- [ ] **Step 2: Update submission handler for file uploads**

In `internal/api/handler/submission.go`, update the `Create` method to handle output-only submissions:

```go
// After decoding the request, set SubmissionType:
submissionType := req.SubmissionType
if submissionType == "" {
    submissionType = "code"
}

sub := &model.Submission{
    // ... existing fields ...
    SubmissionType: submissionType,
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/judge/... -v && go test ./internal/api/handler/... -v`
Expected: Tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/judge/worker.go internal/api/handler/submission.go
git commit -m "feat: implement submit-answer (output-only) problem judging"
```

---

## Task 7: Subtask Scoring

**Files:**
- Modify: `internal/judge/worker.go`

- [ ] **Step 1: Add subtask scoring logic**

In `internal/judge/worker.go`, add a method for subtask evaluation:

```go
// evaluateSubtasks runs test cases grouped by subtask and aggregates scores.
func (wp *WorkerPool) evaluateSubtasks(ctx context.Context, sub *model.Submission, prob *model.Problem, langCfg LanguageConfig) {
    subtasks := prob.GetSubtasks()
    
    // Sort subtask IDs for deterministic order
    var subtaskIDs []int
    for id := range subtasks {
        subtaskIDs = append(subtaskIDs, id)
    }
    sort.Ints(subtaskIDs)
    
    var allResults []model.TestCaseResult
    totalScore := 0
    maxScore := 0
    maxTime := uint64(0)
    maxMemory := uint64(0)
    finalStatus := model.StatusAC
    
    for _, stID := range subtaskIDs {
        cases := subtasks[stID]
        subtaskScore := 0
        subtaskMaxScore := 0
        
        for _, tc := range cases {
            subtaskMaxScore += tc.Score
            maxScore += tc.Score
            
            // Run test case (same as existing logic)
            result := wp.runTestCase(ctx, sub, prob, langCfg, tc)
            allResults = append(allResults, result)
            
            if result.Time > maxTime { maxTime = result.Time }
            if result.Memory > maxMemory { maxMemory = result.Memory }
            
            if result.Status == "ac" {
                subtaskScore += tc.Score
            } else {
                // For 'min' aggregation, one failure = 0 for subtask
                if prob.SubtaskAggregation == "min" {
                    subtaskScore = 0
                    break
                }
                // For 'sum' aggregation, just don't add this case's score
            }
            
            if result.Status != "ac" && finalStatus == model.StatusAC {
                finalStatus = model.SubmissionStatus(result.Status)
            }
        }
        
        totalScore += subtaskScore
    }
    
    percentageScore := 0
    if maxScore > 0 {
        percentageScore = (totalScore * 100) / maxScore
    }
    
    wp.subStore.UpdateResult(ctx, sub.ID, finalStatus, percentageScore, maxTime, maxMemory, "", allResults)
    wp.probStore.UpdateCounts(ctx, sub.ProblemID, true, boolToInt(finalStatus == model.StatusAC))
}
```

- [ ] **Step 2: Update judge() to dispatch to subtask evaluation**

In the main `judge()` function, after compiling and before the test case loop, add:

```go
// If problem has subtasks, use subtask evaluation
if prob.HasSubtasks() && prob.ScoringMode == "partial" {
    wp.evaluateSubtasks(ctx, sub, prob, langCfg)
    return
}
```

- [ ] **Step 3: Extract common test case runner**

Refactor the existing test case loop into a `runTestCase` helper to avoid code duplication between the subtask path and the existing path.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/judge/... -v`
Expected: Tests pass

- [ ] **Step 5: Commit**

```bash
git add internal/judge/worker.go
git commit -m "feat: implement IOI-style subtask scoring with min/sum aggregation"
```

---

## Task 8: Per-Language Resource Limits in Worker

**Files:**
- Modify: `internal/judge/worker.go`

- [ ] **Step 1: Add language limit loading**

In `internal/judge/worker.go`, add a helper to get effective limits:

```go
// getEffectiveLimits returns the time/memory limits for a specific language,
// considering per-language overrides.
func (wp *WorkerPool) getEffectiveLimits(prob *model.Problem, language string) (timeLimitMs int, memoryLimitKB int) {
    timeLimitMs = prob.TimeLimit
    memoryLimitKB = prob.MemoryLimit
    
    // Check for per-language overrides
    for _, ll := range prob.LanguageLimits {
        if ll.LanguageID == language {
            if ll.TimeLimitMs != nil {
                timeLimitMs = *ll.TimeLimitMs
            }
            if ll.MemoryLimitKB != nil {
                memoryLimitKB = *ll.MemoryLimitKB
            }
            break
        }
    }
    
    // Apply language config multipliers
    langCfg := wp.loadLangConfig(language)
    timeLimitMs = int(float64(timeLimitMs) * langCfg.TimeLimitMultiplier)
    memoryLimitKB = int(float64(memoryLimitKB) * langCfg.MemoryLimitMultiplier)
    
    return
}
```

- [ ] **Step 2: Update test case execution to use effective limits**

In the test case loop, replace the hardcoded limit calculation with:

```go
timeLimit, memoryLimit := wp.getEffectiveLimits(prob, sub.Language)
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/judge/... -v`
Expected: Tests pass

- [ ] **Step 4: Commit**

```bash
git add internal/judge/worker.go
git commit -m "feat: apply per-language resource limits with language config multipliers"
```

---

## Task 9: Wiring & Integration

**Files:**
- Modify: `cmd/aioj/main.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Wire LanguageLimitStore in main.go**

In `cmd/aioj/main.go`, add after other store initializations:

```go
langLimitStore := postgres.NewLanguageLimitStore(db)
```

Pass it to the relevant handlers or make it available to the judge worker.

- [ ] **Step 2: Update worker pool constructor**

Update `NewWorkerPool` to accept and store `LanguageLimitStore`:

```go
type WorkerPool struct {
    queue         queue.JudgeQueue
    exec          *executor.Client
    langDir       string
    sem           chan struct{}
    subStore      store.SubmissionStore
    probStore     store.ProblemStore
    langLimitStore store.LanguageLimitStore  // NEW
}
```

- [ ] **Step 3: Run full test suite**

Run: `go test ./... -count=1`
Expected: All 145+ tests pass

- [ ] **Step 4: Build and verify**

Run: `go build ./cmd/aioj`
Expected: Builds without errors

- [ ] **Step 5: Commit**

```bash
git add cmd/aioj/main.go internal/judge/worker.go
git commit -m "feat: wire LanguageLimitStore and integrate all Phase 1 components"
```

---

## Task 10: Frontend Updates

**Files:**
- Modify: `web/src/pages/ProblemDetail.tsx`
- Modify: `web/src/pages/SubmissionDetail.tsx`

- [ ] **Step 1: Add problem type indicator in ProblemDetail.tsx**

In `web/src/pages/ProblemDetail.tsx`, add a badge after the problem title:

```tsx
{/* After the title */}
{problem.interactive && (
  <span className="ml-2 px-2 py-1 text-xs font-semibold bg-purple-100 text-purple-800 rounded">
    Interactive
  </span>
)}
{problem.submission_type === 'output' && (
  <span className="ml-2 px-2 py-1 text-xs font-semibold bg-orange-100 text-orange-800 rounded">
    Output Only
  </span>
)}
```

- [ ] **Step 2: Show subtask scores in SubmissionDetail.tsx**

In `web/src/pages/SubmissionDetail.tsx`, update the Tests tab to group results by subtask when available:

```tsx
{/* Group by subtask if subtask_id present in results */}
const hasSubtasks = judgeResult?.some(r => r.subtask_id > 0);

if (hasSubtasks) {
  // Group and display by subtask
  const subtasks = groupBySubtask(judgeResult);
  return (
    <div className="space-y-4">
      {subtasks.map(([id, cases]) => (
        <div key={id} className="border rounded p-3">
          <h4>Subtask {id}: {getSubtaskScore(cases)}/{getSubtaskMax(cases)}</h4>
          {cases.map((c, i) => (
            <div key={i}>{c.case_name}: {c.status} ({c.time}ms, {c.memory}KB)</div>
          ))}
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 3: Build frontend**

Run: `cd web && npm run build`
Expected: TypeScript compiles, no errors

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/ProblemDetail.tsx web/src/pages/SubmissionDetail.tsx
git commit -m "feat: add problem type indicators and subtask score display in frontend"
```

---

## Task 11: Integration Tests

**Files:**
- Create: `internal/judge/worker_integration_test.go`

- [ ] **Step 1: Write integration test for subtask scoring**

```go
package judge

import (
    "testing"
    
    "github.com/AIOJ/AIOJ/internal/model"
)

func TestEvaluateSubtasks_MinAggregation(t *testing.T) {
    prob := &model.Problem{
        ScoringMode:        "partial",
        SubtaskAggregation: "min",
        TestCaseScore: []model.TestCaseScore{
            {InputName: "1.in", OutputName: "1.out", Score: 10, SubtaskID: 1},
            {InputName: "2.in", OutputName: "2.out", Score: 10, SubtaskID: 1},
            {InputName: "3.in", OutputName: "3.out", Score: 20, SubtaskID: 2},
        },
    }
    
    subtasks := prob.GetSubtasks()
    if len(subtasks) != 2 {
        t.Fatalf("expected 2 subtasks, got %d", len(subtasks))
    }
    
    // Subtask 1 has 2 cases, subtask 2 has 1 case
    if len(subtasks[1]) != 2 {
        t.Errorf("expected 2 cases in subtask 1, got %d", len(subtasks[1]))
    }
    if len(subtasks[2]) != 1 {
        t.Errorf("expected 1 case in subtask 2, got %d", len(subtasks[2]))
    }
}

func TestEvaluateSubtasks_SumAggregation(t *testing.T) {
    prob := &model.Problem{
        ScoringMode:        "partial",
        SubtaskAggregation: "sum",
        TestCaseScore: []model.TestCaseScore{
            {InputName: "1.in", OutputName: "1.out", Score: 10, SubtaskID: 1},
            {InputName: "2.in", OutputName: "2.out", Score: 15, SubtaskID: 1},
        },
    }
    
    subtasks := prob.GetSubtasks()
    // Sum aggregation: both cases contribute independently
    totalScore := 0
    for _, tc := range subtasks[1] {
        totalScore += tc.Score
    }
    if totalScore != 25 {
        t.Errorf("expected total score 25, got %d", totalScore)
    }
}
```

- [ ] **Step 2: Run integration tests**

Run: `go test ./internal/judge/ -v -run TestEvaluateSubtasks`
Expected: All tests pass

- [ ] **Step 3: Commit**

```bash
git add internal/judge/worker_integration_test.go
git commit -m "test: add integration tests for subtask scoring logic"
```

---

## Verification Checklist

After completing all tasks:

- [ ] All migrations apply cleanly (`make migrate-up`)
- [ ] All backend tests pass (`go test ./... -count=1`)
- [ ] Frontend builds without errors (`cd web && npm run build`)
- [ ] Interactive problems can be created and judged (manual test with a simple interactor)
- [ ] Submit-answer problems accept file uploads and compare output
- [ ] Subtask scoring works with both `min` and `sum` aggregation
- [ ] All 8 checker types work correctly
- [ ] Per-language limits override problem defaults correctly
- [ ] No regressions in existing functionality (standard problems still judge correctly)
