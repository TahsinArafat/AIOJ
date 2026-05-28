# AIOJ Superiority: Comprehensive Design Spec

> **Status:** Approved  
> **Last updated:** 2026-05-29  
> **Author:** AIOJ Team  
> **Approach:** Foundation-First (Judge Engine → Platform → Scale)

**Goal:** Transform AIOJ from a basic online judge into the most capable open-source Go-based competitive programming platform — surpassing Hydro, HUSTOJ, DMOJ, and Judge0 in judging depth, problem ecosystem, and deployment simplicity.

**North Stars:** Faster, Reliable, Scalable, Stable.

---

## Executive Summary

After auditing 8 reference projects (Hydro, HUSTOJ, Judge0, DMOJ, Cato, go-judge-system, judge-server, VNOJ), we identified **15 gaps** preventing AIOJ from achieving superiority. This spec addresses all of them across 7 phases:

| Phase | Scope | Duration |
|-------|-------|----------|
| 1 | Judging Engine v2 (interactive, submit-answer, subtask scoring, checkers, per-language limits) | 4-6 weeks |
| 2 | Problem Ecosystem (FPS import/export, Hydro format) | 2-3 weeks |
| 3 | Contest Depth (pluggable format registry, IOI/AtCoder/Codeforces formats) | 3-4 weeks |
| 4 | Community Platform (organizations, classes, training plans) | 4-5 weeks |
| 5 | Quality Assurance (plagiarism detection, problem recommendations) | 2-3 weeks |
| 6 | Scale & Reliability (Redis, distributed judge, read replicas) | 3-4 weeks |
| 7 | Polish (monitoring, performance, load testing) | 2-3 weeks |

**Total: 20-28 weeks (5-7 months)**

---

## 1. Judging Engine v2 — Interactive & Submit-Answer Problems

### 1.1 Interactive Problems

**How it works (Hydro/Codeforces model):**
1. Problem setter provides an **interactor** program (compiled binary or script)
2. When a contestant's solution runs, the interactor communicates with it via stdin/stdout pipes
3. The interactor sends input, reads the contestant's output, and sends the next input
4. Final verdict comes from the interactor (AC/WA/FAIL)

**Protocol:**
```
Interactor ←→ Contestant Program
     ↓
  Verdict (AC/WA/FAIL)
  + optional message
```

**Implementation:**
- New problem type field: `problem_type ENUM('standard', 'interactive', 'submit_answer')`
- Interactor source stored alongside problem test data
- Judge worker spawns both programs, connects via pipes
- go-judge already supports `PipeInput` — we wire the interactor's stdout to contestant's stdin and vice versa
- Timeout applies to total interaction, not just execution

**File changes:**
- `internal/model/problem.go` — add `ProblemType` field
- `internal/judge/worker.go` — add interactive execution path
- `internal/judge/executor/executor.go` — pipe interactor ↔ contestant
- `web/src/pages/ProblemDetail.tsx` — UI indicator for interactive problems
- Database migration for new column

### 1.2 Submit-Answer (Output-Only) Problems

**How it works:**
1. Problem provides input files and (optionally) expected output
2. Contestant uploads **output files** directly (no source code submission)
3. Judge compares uploaded output against expected output using the configured checker
4. Partial scoring based on test case results

**Implementation:**
- New submission type: `submission_type ENUM('code', 'output')`
- When type is `output`, the `source_code` field contains the uploaded output file content
- Judge skips compilation, goes directly to output comparison
- Frontend shows file upload UI instead of code editor

**File changes:**
- `internal/model/submission.go` — add `SubmissionType` field
- `internal/judge/worker.go` — skip compilation for output-only submissions
- `internal/api/handler/submission.go` — accept file uploads
- `web/src/pages/SubmitProblem.tsx` — conditional UI (editor vs file upload)

### 1.3 Verdict Flow (Updated)

```
Standard:      Compile → Run → Compare → Verdict
Interactive:   Compile → Run(interactor ↔ program) → Interactor verdict
Submit-Answer: Upload → Compare → Verdict
```

---

## 2. Subtask/Batch Scoring (IOI Format)

### 2.1 Simplified Problem Structure

Flat test case list with optional subtask grouping. No dependency chains for v1. Just batch scoring.

```sql
-- Add subtask_id to existing test cases (nullable)
ALTER TABLE problem_testcases 
    ADD COLUMN subtask_id INTEGER,
    ADD COLUMN subtask_score INTEGER;

-- Problem-level config
ALTER TABLE problems 
    ADD COLUMN scoring_mode VARCHAR(16) NOT NULL DEFAULT 'batch', -- 'batch' or 'complete'
    ADD COLUMN subtask_aggregation VARCHAR(8) NOT NULL DEFAULT 'min'; -- 'min' or 'sum'
```

**Example:** A problem with 9 test cases split into 3 subtasks:
```
Cases: 1.in, 2.in, 3.in → subtask_id=1, subtask_score=10 each (30 total)
Cases: 4.in, 5.in, 6.in → subtask_id=2, subtask_score=15 each (45 total)
Cases: 7.in, 8.in, 9.in → subtask_id=3, subtask_score=25 each (75 total)
Max score: 150
```

### 2.2 Scoring Rules

**Two modes only (v1):**

| Mode | Behavior | When to use |
|------|----------|-------------|
| `partial` | Each subtask scored independently. Final = sum of subtask scores. | IOI-style contests |
| `all_or_nothing` | All-or-nothing per problem. AC if all cases pass, 0 otherwise. | ACM-style contests |

**Subtask aggregation:**
- `min`: Subtask score = minimum case score in the subtask (all must pass for full points)
- `sum`: Subtask score = sum of individual case scores (partial credit per case)

### 2.3 Judge Flow

```
1. For each subtask:
   a. Run all cases in the subtask (parallel)
   b. Collect per-case results: {case_id, score, time, memory, status}
   c. Aggregate subtask score:
      - min: subtask_score = min(case_scores)
      - sum: subtask_score = sum(case_scores)
2. Total score = sum of all subtask scores
3. If scoring_mode == 'complete':
   - If any case failed → total_score = 0
   - Else → total_score = max_score
```

### 2.4 Frontend Display

**During contest (OI mode):**
```
Subtask 1: 30/30 ✓
Subtask 2: 30/45 (3/3 passed but score capped at min)
Subtask 3: 0/75 ✗ (failed case 7)
Total: 60/150
```

**After contest (upsolving):**
```
Test 1: ✓ 10/10 (0.12s, 8MB)
Test 2: ✓ 10/10 (0.08s, 6MB)
Test 3: ✓ 10/10 (0.15s, 9MB)
Test 4: ✓ 15/15 (0.22s, 12MB)
Test 5: ✓ 15/15 (0.18s, 10MB)
Test 6: ✓ 15/15 (0.25s, 14MB)
Test 7: ✗ 0/25 (WA, 0.31s, 16MB)
Test 8: - 0/25 (skipped)
Test 9: - 0/25 (skipped)
Total: 60/150
```

---

## 3. Checker Types & Per-Language Resource Limits

### 3.1 Expanded Checker Types

**Current (3 checkers):**
- ExactChecker — byte-exact comparison
- LinesChecker — line-by-line (order-independent)
- FloatChecker — numeric tolerance

**Adding (5 more checkers):**

| Checker | Behavior | Use Case |
|---------|----------|----------|
| `FloatAbsolute` | `|expected - actual| <= epsilon` | Physics problems |
| `FloatRelative` | `|expected - actual| / |expected| <= epsilon` | Large numbers |
| `Sorted` | Sort both outputs, compare | Set equality problems |
| `Unordered` | Compare as multisets (count occurrences) | Permutation problems |
| `ByteIdentical` | Binary-exact comparison (no whitespace trim) | Binary output problems |

**Implementation:**
```go
// internal/judge/checker/checker.go

type CheckerType string

const (
    CheckerExact         CheckerType = "exact"
    CheckerLines         CheckerType = "lines"
    CheckerFloat         CheckerType = "float"
    CheckerFloatAbs      CheckerType = "float_absolute"
    CheckerFloatRel      CheckerType = "float_relative"
    CheckerSorted        CheckerType = "sorted"
    CheckerUnordered     CheckerType = "unordered"
    CheckerByteIdentical CheckerType = "byte_identical"
)

// ProblemTestCase now has:
type ProblemTestCase struct {
    // ... existing fields ...
    CheckerType CheckerType `json:"checker_type"` // per-case override
}
```

**Config format (problem testdata):**
```yaml
# config.yaml
checker: float_absolute  # default checker for all cases
cases:
  - input: 1.in
    output: 1.out
    checker: exact  # override for specific case
  - input: 2.in
    output: 2.out
    # uses default (float_absolute)
```

### 3.2 Per-Language Resource Limits

**Problem:** Some languages are slower (Python 3x slower than C++) and use more memory. One global time/memory limit doesn't work fairly.

**Solution:** Per-language multipliers on the problem's base limits.

```sql
CREATE TABLE language_limits (
    problem_id UUID NOT NULL REFERENCES problems(id),
    language_id VARCHAR(64) NOT NULL,
    time_limit_ms INTEGER,      -- NULL = use problem default
    memory_limit_kb INTEGER,    -- NULL = use problem default
    PRIMARY KEY (problem_id, language_id)
);
```

**Example:**
```yaml
# Problem: time_limit=1000ms, memory_limit=256MB
# Language limits:
#   python3: time_multiplier=3.0 (3000ms), memory_multiplier=1.5 (384MB)
#   java: time_multiplier=2.0 (2000ms), memory_multiplier=2.0 (512MB)
#   cpp: no override (uses base limits)
```

**Judge worker logic:**
```go
func getTimeLimit(problem Problem, language string) int {
    limit := problem.TimeLimitMs
    if langLimit, ok := problem.LanguageLimits[language]; ok && langLimit.TimeLimitMs != nil {
        limit = *langLimit.TimeLimitMs
    }
    return limit
}
```

**Frontend:** Show language-specific limits on the problem page:
```
Time Limit: 1000ms (C++), 3000ms (Python), 2000ms (Java)
Memory Limit: 256MB
```

---

## 4. FPS Import/Export & Problem Package Format

### 4.1 FPS (Free Problem Set) Format

FPS is the standard XML interchange format for competitive programming problems. HUSTOJ invented it, and Hydro/HUSTOJ/OJ all support it. Supporting FPS means instant access to **thousands of existing problems**.

**FPS XML Structure:**
```xml
<?xml version="1.0" encoding="UTF-8"?>
<fps version="1.2">
  <problem>
    <title>Two Sum</title>
    <time_limit>1000</time_limit>
    <memory_limit>256</memory_limit>
    <description>Given an array...</description>
    <input>First line contains n...</input>
    <output>Output the sum...</output>
    <sample_input>
      3
      1 2 3
    </sample_input>
    <sample_output>
      6
    </sample_output>
    <test_input>4\n1 2 3 4\n</test_input>
    <test_output>10\n</test_output>
    <test_input>2\n5 5\n</test_input>
    <test_output>10\n</test_output>
    <hint>Use a hashmap...</hint>
    <source>Codeforces 4A</source>
    <tags>implementation,math</tags>
  </problem>
</fps>
```

**Key fields:**
- `test_input` / `test_output` — inline test case data (not files)
- Multiple test cases supported via repeated elements
- SPJ supported via `spj` element (checker source code)
- Multiple problems per XML file

### 4.2 Implementation

**Backend (Go):**
```
internal/
├── fps/
│   ├── parser.go      # FPS XML → Problem model
│   ├── generator.go   # Problem model → FPS XML
│   └── types.go       # FPS XML structs
├── api/handler/
│   └── import.go      # POST /api/problems/import (multipart/form-data)
```

**API Endpoints:**
```
POST /api/problems/import     — Upload FPS ZIP/XML, create problems
GET  /api/problems/:id/export — Download problem as FPS ZIP
GET  /api/problems/export/all — Export all problems as FPS ZIP (admin)
```

**Import flow:**
1. User uploads FPS ZIP (contains `problem.xml` + test data files)
2. Parser extracts XML, creates Problem record
3. Test cases stored in the problem's testdata directory
4. If SPJ present, compile and store checker
5. Return import result (problems created, errors)

**Export flow:**
1. Read problem data + test cases from storage
2. Generate FPS XML
3. Package as ZIP (XML + test files)
4. Return download

### 4.3 Hydro Format Support (Bonus)

Hydro uses a similar YAML-based config format. Since we already use YAML for language configs, supporting Hydro format is straightforward:

```yaml
# Hydro problem config.yaml
title: Two Sum
config:
  time: 1000
  memory: 256
  type: default
  checker:
    name: std
  subtasks:
    - score: 100
      cases:
        - input: 1.in
          output: 1.out
```

**Implementation:** Add `internal/fps/hydro_parser.go` for Hydro format parsing.

### 4.4 Import UI

**Admin Panel → Problem Import:**
```
┌─────────────────────────────────────────┐
│ Import Problems                         │
│                                         │
│ Format: [FPS XML ▼]  or  [Hydro YAML ▼]│
│                                         │
│ File: [Choose file...]                  │
│                                         │
│ [Import]                                │
│                                         │
│ Results:                                │
│ ✓ Problem "Two Sum" created (ID: 42)    │
│ ✓ Problem "Three Sum" created (ID: 43)  │
│ ✗ Problem "Bad Problem" — missing output│
│                                         │
│ Total: 2 imported, 1 failed             │
└─────────────────────────────────────────┘
```

---

## 5. Pluggable Contest Format Registry

### 5.1 The Pattern (from DMOJ)

Instead of hardcoding contest scoring logic, use a **registry pattern** where each format is a self-contained module:

```go
// internal/contest/format/registry.go

type ContestFormat interface {
    Name() string
    UpdateParticipation(p *Participation, s *Submission)
    DisplayUserProblem(p *Participation, problemID uuid.UUID) FormattedResult
    DisplayParticipationResult(p *Participation) FormattedResult
}

var formats = map[string]ContestFormat{}

func Register(name string, f ContestFormat) {
    formats[name] = f
}

func Get(name string) (ContestFormat, bool) {
    f, ok := formats[name]
    return f, ok
}
```

### 5.2 Built-in Formats (5)

**1. ACM/ICPC (already implemented)**
```go
type ACMFormat struct{}

func (f *ACMFormat) UpdateParticipation(p *Participation, s *Submission) {
    // Penalty = first_ac_time + 20 * wrong_attempts
    // Solve count = problems with at least one AC
}
```

**2. OI (already implemented)**
```go
type OIFormat struct{}

func (f *OIFormat) UpdateParticipation(p *Participation, s *Submission) {
    // Score = max score across all submissions per problem
    // No penalty
}
```

**3. IOI (new)**
```go
type IOIFormat struct{}

func (f *IOIFormat) UpdateParticipation(p *Participation, s *Submission) {
    // Score = subtask-based scoring
    // Partial credit per subtask
    // Best submission per problem
}
```

**4. AtCoder (new)**
```go
type AtCoderFormat struct{}

func (f *AtCoderFormat) UpdateParticipation(p *Participation, s *Submission) {
    // Similar to ACM but:
    // - Penalty = time of first AC (no 20-min per wrong attempt)
    // - Tiebreak: last AC time earlier wins
}
```

**5. Codeforces (new)**
```go
type CodeforcesFormat struct{}

func (f *CodeforcesFormat) UpdateParticipation(p *Participation, s *Submission) {
    // Dynamic scoring: points decrease over time
    // Hacking phase extends scoring window
    // Penalty similar to ACM
}
```

### 5.3 Database Changes

```sql
ALTER TABLE contests 
    ADD COLUMN format VARCHAR(32) NOT NULL DEFAULT 'acm',
    ADD COLUMN format_config JSONB DEFAULT '{}';
```

**Format-specific config (JSONB):**
```json
// ACM format_config
{
  "penalty_per_wrong": 20,
  "first_solve_bonus": false
}

// IOI format_config
{
  "subtask_aggregation": "min",
  "best_submission": true
}

// Codeforces format_config
{
  "hacking_phase_hours": 12,
  "dynamic_scoring": true
}
```

### 5.4 Adding a New Format (Developer Experience)

```go
// internal/contest/format/custom.go

type MyCustomFormat struct{}

func (f *MyCustomFormat) Name() string { return "my_custom" }

func (f *MyCustomFormat) UpdateParticipation(p *Participation, s *Submission) {
    // Custom scoring logic
}

func init() {
    Register("my_custom", &MyCustomFormat{})
}
```

That's it. No config file changes, no database schema changes. Just implement the interface and register.

### 5.5 Frontend

**Contest Creation:**
```
Contest Format: [ACM ▼]
  └─ Penalty per wrong submission: [20] minutes

Contest Format: [IOI ▼]
  └─ Aggregation: [min ▼]
  └─ Best submission: [✓]

Contest Format: [Codeforces ▼]
  └─ Hacking phase: [12] hours
  └─ Dynamic scoring: [✓]
```

---

## 6. Organization/Class Hierarchy & Training Plans

### 6.1 Organization System (Hydro/DMOJ Model)

**Hierarchy:**
```
Organization (e.g., "MIT", "Code Club BD")
├── Class (e.g., "CS101 Fall 2026", "Advanced Training")
│   ├── Students (users enrolled in class)
│   └── Class-specific contests
└── Organization contests (open to all org members)
```

**Database Schema:**
```sql
CREATE TABLE organizations (
    id UUID PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    slug VARCHAR(128) UNIQUE NOT NULL,
    description TEXT,
    avatar_url VARCHAR(512),
    is_public BOOLEAN NOT NULL DEFAULT true,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE organization_members (
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    role VARCHAR(16) NOT NULL DEFAULT 'member', -- owner, admin, member
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, user_id)
);

CREATE TABLE classes (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    description TEXT,
    invite_code VARCHAR(32) UNIQUE, -- students join via code
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE class_members (
    class_id UUID NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    role VARCHAR(16) NOT NULL DEFAULT 'student', -- teacher, student
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (class_id, user_id)
);
```

**API Endpoints:**
```
POST   /api/organizations              — Create org
GET    /api/organizations              — List orgs (public)
GET    /api/organizations/:slug        — Org detail
POST   /api/organizations/:slug/join   — Join org
DELETE /api/organizations/:slug/leave  — Leave org

POST   /api/organizations/:slug/classes              — Create class
GET    /api/organizations/:slug/classes              — List classes
POST   /api/organizations/:slug/classes/:id/join     — Join class (via invite code)
DELETE /api/organizations/:slug/classes/:id/leave    — Leave class

GET    /api/organizations/:slug/contests             — Org contests
POST   /api/organizations/:slug/contests             — Create org contest
GET    /api/organizations/:slug/classes/:id/contests — Class contests
```

**Contest Restriction:**
```sql
ALTER TABLE contests 
    ADD COLUMN org_id UUID REFERENCES organizations(id),
    ADD COLUMN class_id UUID REFERENCES classes(id);
```

If `org_id` is set → only org members can see/register.
If `class_id` is set → only class members can see/register.

### 6.2 Training Plans (Hydro Model)

A **training plan** is a curated sequence of problems with progress tracking.

**Database Schema:**
```sql
CREATE TABLE training_plans (
    id UUID PRIMARY KEY,
    title VARCHAR(256) NOT NULL,
    description TEXT,
    difficulty VARCHAR(16), -- easy, medium, hard
    tags TEXT[],
    created_by UUID NOT NULL REFERENCES users(id),
    is_public BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE training_plan_sections (
    id UUID PRIMARY KEY,
    plan_id UUID NOT NULL REFERENCES training_plans(id) ON DELETE CASCADE,
    title VARCHAR(256) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE training_plan_problems (
    section_id UUID NOT NULL REFERENCES training_plan_sections(id) ON DELETE CASCADE,
    problem_id UUID NOT NULL REFERENCES problems(id),
    sort_order INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (section_id, problem_id)
);

CREATE TABLE training_plan_progress (
    plan_id UUID NOT NULL REFERENCES training_plans(id),
    user_id UUID NOT NULL REFERENCES users(id),
    completed_problems UUID[] NOT NULL DEFAULT '{}',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (plan_id, user_id)
);
```

**API Endpoints:**
```
GET    /api/training                    — List training plans
POST   /api/training                    — Create plan (admin)
GET    /api/training/:id                — Plan detail with sections
POST   /api/training/:id/enroll         — Enroll in plan
GET    /api/training/:id/progress       — My progress
PATCH  /api/training/:id/progress       — Update progress (mark problem done)
```

**Frontend:**
```
Training Plans
├── "Algorithmic Problem Solving" (32 problems)
│   ├── Section 1: Basics (8 problems) — ✓ 8/8 complete
│   ├── Section 2: Sorting (8 problems) — ✓ 6/8 complete
│   ├── Section 3: Graphs (8 problems) — ○ 2/8 complete
│   └── Section 4: DP (8 problems) — ○ 0/8 complete
│   Progress: 16/32 (50%)
│
├── "ICPC Training Camp" (48 problems)
│   ├── ...
```

---

## 7. Plagiarism Detection & Problem Recommendations

### 7.1 Plagiarism Detection (MOSS/SIM Integration)

**Two approaches:**

| Approach | How it works | Pros | Cons |
|----------|-------------|------|------|
| **SIM** (HUSTOJ) | C utility, compares code structure | Fast, lightweight, no external deps | Less accurate, language-limited |
| **MOSS** (Stanford) | Sends code to Stanford servers, returns similarity % | Industry standard, very accurate | Requires internet, privacy concerns |

**Recommendation: Start with SIM (self-hosted), add MOSS as optional.**

**SIM Integration:**
```go
// internal/plagiarism/sim.go

type SIMChecker struct {
    Threshold float64 // default 0.7 (70% similarity)
}

func (s *SIMChecker) Compare(code1, code2 []byte, language string) (SimilarityResult, error) {
    // Write temp files
    // Run: sim -l <language> -t <threshold> file1 file2
    // Parse output
    return SimilarityResult{
        Similarity: 0.85,
        IsPlagiarism: true,
    }, nil
}
```

**API Endpoints:**
```
POST /api/contests/:id/plagiarism         — Run plagiarism check on contest
GET  /api/contests/:id/plagiarism/results  — Get results
GET  /api/problems/:id/plagiarism          — Run on problem submissions
```

**UI (Admin Panel):**
```
Plagiarism Report — Contest #42
┌─────────────────────────────────────────────────────┐
│ Pair                        │ Similarity │ Language  │
├─────────────────────────────┼────────────┼───────────┤
│ alice.cpp ↔ bob.cpp         │ 87%        │ C++       │
│ carol.py ↔ dave.py          │ 72%        │ Python    │
│ eve.java ↔ frank.java       │ 45%        │ Java      │
└─────────────────────────────────────────────────────┘

[View Side-by-Side] [Flag for Review] [Ignore]
```

**Storage:**
```sql
CREATE TABLE plagiarism_reports (
    id UUID PRIMARY KEY,
    contest_id UUID REFERENCES contests(id),
    problem_id UUID REFERENCES problems(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
);

CREATE TABLE plagiarism_pairs (
    id UUID PRIMARY KEY,
    report_id UUID NOT NULL REFERENCES plagiarism_reports(id),
    submission1_id UUID NOT NULL REFERENCES submissions(id),
    submission2_id UUID NOT NULL REFERENCES submissions(id),
    similarity FLOAT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' -- pending, flagged, ignored
);
```

### 7.2 Problem Recommendations

**Algorithm:** Recommend problems based on:
1. **Difficulty rating** — problems slightly above user's current level
2. **Tags** — problems matching user's weak areas (tags they haven't solved many of)
3. **Recency** — newer problems preferred
4. **Popularity** — problems with higher solve rates among similar-level users

**Implementation:**
```go
// internal/recommendation/service.go

type RecommendationService struct {
    store Store
}

func (s *RecommendationService) GetRecommendations(userID uuid.UUID, limit int) ([]Problem, error) {
    user := s.store.GetUser(userID)
    
    // Get user's solved problems
    solved := s.store.GetSolvedProblems(userID)
    
    // Get user's tag distribution
    tagStats := s.store.GetUserTagStats(userID)
    
    // Find weak tags (solved < average)
    weakTags := findWeakTags(tagStats)
    
    // Query: unsolved problems, difficulty ~user+100, prioritize weak tags
    candidates := s.store.GetProblems(ProblemFilter{
        UnsolvedBy: userID,
        DifficultyRange: [user.Rating-200, user.Rating+100],
        Tags: weakTags,
        Limit: limit * 3,
    })
    
    // Score and rank
    scored := scoreProblems(candidates, user, tagStats)
    
    return topN(scored, limit), nil
}
```

**API Endpoints:**
```
GET /api/recommendations              — Get personalized recommendations
GET /api/recommendations?tag=dp       — Get recommendations for specific tag
GET /api/recommendations?difficulty=3 — Get recommendations at difficulty level
```

**Frontend (Problem List sidebar):**
```
Recommended for You
├── Two Sum (Easy) — Math
├── Longest Substring (Medium) — Strings, DP
├── Merge Intervals (Medium) — Sorting
└── Binary Tree Max Path (Hard) — Trees, DFS
[Show More]
```

---

## 8. Scale & Reliability

### 8.1 Redis Integration

**Use cases:**
1. **Caching** — hot data (problem lists, user profiles, contest scoreboards)
2. **Rate limiting** — per-user API rate limits
3. **Session store** — JWT refresh token blacklisting
4. **Distributed judge queue** — replace in-memory queue

**Implementation:**
```go
// internal/cache/redis.go

type RedisCache struct {
    client *redis.Client
    ttl    time.Duration
}

func (c *RedisCache) Get(key string, dest interface{}) error {
    data, err := c.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return ErrCacheMiss
    }
    return json.Unmarshal(data, dest)
}

func (c *RedisCache) Set(key string, value interface{}, ttl time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil { return err }
    return c.client.Set(ctx, key, data, ttl).Err()
}
```

**Cache strategy:**
| Data | TTL | Invalidation |
|------|-----|-------------|
| Problem list | 5 min | On problem create/update |
| User profile | 10 min | On profile update |
| Contest scoreboard | 30 sec | On submission judged |
| Language list | 1 hour | Never (static) |
| Rating history | 5 min | On contest end |

### 8.2 Distributed Judge Worker

**Current:** Single binary, in-memory queue, one judge worker.

**Target:** Extract judge to separate process, HTTP-based task queue.

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│  API Server  │────▶│  Redis Queue  │◀────│  Judge Worker │
│  (Go binary) │     │  (tasks)      │     │  (Go binary)  │
└─────────────┘     └──────────────┘     └──────────────┘
       │                                        │
       │                                        │
       ▼                                        ▼
┌─────────────┐                         ┌──────────────┐
│  PostgreSQL  │                         │  go-judge    │
└─────────────┘                         └──────────────┘
```

**Changes:**
```go
// internal/queue/redis.go (new)

type RedisQueue struct {
    client    *redis.Client
    queueName string
}

func (q *RedisQueue) Enqueue(ctx context.Context, submissionID uuid.UUID) error {
    return q.client.LPush(ctx, q.queueName, submissionID.String()).Err()
}

func (q *RedisQueue) Dequeue(ctx context.Context) (uuid.UUID, error) {
    result, err := q.client.BRPop(ctx, 5*time.Second, q.queueName).Result()
    if err != nil { return uuid.Nil, err }
    return uuid.Parse(result[1])
}
```

**Docker Compose changes:**
```yaml
services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
  
  judge-worker:
    build: .
    command: /app/aioj --mode=judge-worker
    depends_on:
      - redis
      - postgres
    environment:
      - REDIS_URL=redis:6379
      - JUDGE_ENDPOINT=http://go-judge:5050
    deploy:
      replicas: 2  # scale horizontally
```

### 8.3 Database Read Replicas

**For read-heavy operations** (problem lists, scoreboards, rankings):

```go
// internal/store/postgres/readonly.go

type ReadOnlyDB struct {
    primary *sql.DB
    replica  *sql.DB
}

func (db *ReadOnlyDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
    // Reads go to replica
    return db.replica.Query(query, args...)
}

func (db *ReadOnlyDB) Exec(query string, args ...interface{}) (sql.Result, error) {
    // Writes go to primary
    return db.primary.Exec(query, args...)
}
```

**PostgreSQL streaming replication:**
```yaml
services:
  postgres-primary:
    image: postgres:18-alpine
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
  
  postgres-replica:
    image: postgres:18-alpine
    environment:
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    command: >
      bash -c "pg_basebackup -h postgres-primary -U aioj -D /var/lib/postgresql/data -R"
```

### 8.4 Graceful Degradation

```go
// internal/middleware/circuit_breaker.go

func CircuitBreaker(threshold int, timeout time.Duration) Middleware {
    var failures int32
    var lastFailure time.Time
    
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if atomic.LoadInt32(&failures) >= int32(threshold) {
                if time.Since(lastFailure) < timeout {
                    // Circuit open — return cached response or 503
                    http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
                    return
                }
                // Half-open — allow one request through
                atomic.StoreInt32(&failures, 0)
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

---

## 9. Implementation Order & Effort

### 9.1 Master Roadmap

| Phase | Scope | Duration | Dependencies |
|-------|-------|----------|-------------|
| **Phase 1: Judging Engine v2** | Interactive, submit-answer, subtask scoring, checkers, per-language limits | 4-6 weeks | None |
| **Phase 2: Problem Ecosystem** | FPS import/export, Hydro format support | 2-3 weeks | Phase 1 (subtask scoring) |
| **Phase 3: Contest Depth** | Pluggable format registry, IOI/AtCoder/Codeforces formats | 3-4 weeks | Phase 1 (subtask scoring) |
| **Phase 4: Community Platform** | Organizations, classes, training plans | 4-5 weeks | None |
| **Phase 5: Quality Assurance** | Plagiarism detection (SIM), problem recommendations | 2-3 weeks | Phase 4 (user data) |
| **Phase 6: Scale & Reliability** | Redis, distributed judge, read replicas, circuit breaker | 3-4 weeks | None |
| **Phase 7: Polish** | Monitoring dashboards, performance optimization, load testing | 2-3 weeks | Phase 6 |

**Total: 20-28 weeks (5-7 months)**

### 9.2 What Each Phase Delivers

**Phase 1 → "Best-in-class Go judge engine"**
- Interactive problems (like Codeforces)
- Submit-answer problems (like DMOJ)
- IOI-style subtask scoring
- 8 checker types (like DMOJ/Hydro)
- Fair time/memory limits per language

**Phase 2 → "Instant problem library"**
- Import thousands of problems from HUSTOJ/Hydro
- Export your problems for others to use
- Problem interchange standard (FPS)

**Phase 3 → "Contest format flexibility"**
- Any contest format via plugin system
- IOI, AtCoder, Codeforces scoring
- Custom formats without code changes to core

**Phase 4 → "Institutional deployment"**
- Schools/universities can host their own instances
- Class management with invite codes
- Curated training plans

**Phase 5 → "Fair play & learning"**
- Plagiarism detection
- Personalized problem recommendations

**Phase 6 → "Production-ready scale"**
- Horizontal scaling (multiple judge workers)
- Redis caching for performance
- Database read replicas
- Graceful degradation

**Phase 7 → "Operational excellence"**
- Monitoring dashboards
- Performance benchmarks
- Load testing

### 9.3 Quick Wins (Ship First)

These can be implemented in 1-2 weeks each and have high impact:

1. **FPS Import** — instant access to thousands of problems
2. **Per-language limits** — fairer judging, easy to implement
3. **More checker types** — 5 new checkers in ~1 week
4. **Training plans** — community engagement, moderate effort

### 9.4 Risk Mitigation

| Risk | Mitigation |
|------|-----------|
| Interactive judging complexity | Start with simple pipe-based protocol, test with known interactors |
| FPS format edge cases | Validate against HUSTOJ test suite, handle malformed XML gracefully |
| Redis single point of failure | Use Redis Sentinel for HA, or start with in-memory fallback |
| Distributed judge consistency | Use Redis as source of truth, implement idempotent submissions |
| Organization permission complexity | Start with simple owner/admin/member, add granular permissions later |

---

## Appendix A: Reference Project Comparison

| Dimension | AIOJ (Current) | Hydro | HUSTOJ | DMOJ | Judge0 |
|-----------|----------------|-------|--------|------|--------|
| **Stack** | Go + React 19 | Node.js + React 19 | PHP + MySQL | Django + Jinja2 | Ruby on Rails |
| **Database** | PostgreSQL | MongoDB | MySQL | MySQL/SQLite | PostgreSQL |
| **Sandbox** | go-judge | go-judge | ptrace/cgroups | cptbox/ptrace | Isolate |
| **Problem Types** | Standard + SPJ | 6 types | 5 types | Standard + SPJ | Standard only |
| **Contest Formats** | ACM/OI | 6 formats | ACM/OI/NOIP | 5 formats | None |
| **Checker Types** | 3 | 8+ | 4 | 8 | 1 (diff only) |
| **FPS Import** | No | Yes | Yes (native) | No | No |
| **Organizations** | No | Yes (Domains) | No | Yes | No |
| **Plagiarism** | No | No | Yes (SIM) | Yes (MOSS) | No |
| **Multi-tenancy** | No | Yes | Yes (SaaS) | No | No |
| **Deploy Complexity** | Low (docker compose) | Medium | High | High | Low |

## Appendix B: Database Migration Summary

| Migration | Tables Added | Phase |
|-----------|-------------|-------|
| 000017_problem_types | `problem_type` column on problems | 1 |
| 000018_subtask_scoring | `subtasks`, `subtask_cases` tables | 1 |
| 000019_checker_types | `checker_type` column on test cases | 1 |
| 000020_language_limits | `language_limits` table | 1 |
| 000021_fps_import | `fps_import_jobs` table | 2 |
| 000022_contest_formats | `format`, `format_config` on contests | 3 |
| 000023_organizations | `organizations`, `organization_members` tables | 4 |
| 000024_classes | `classes`, `class_members` tables | 4 |
| 000025_training | `training_plans`, `training_plan_sections`, etc. | 4 |
| 000026_plagiarism | `plagiarism_reports`, `plagiarism_pairs` tables | 5 |
| 000027_recommendations | `problem_recommendations` table | 5 |

## Appendix C: API Endpoint Summary

### New Endpoints (Phase 1-7)

| Method | Endpoint | Phase | Description |
|--------|----------|-------|-------------|
| POST | `/api/problems/import` | 2 | Import FPS/Hydro problems |
| GET | `/api/problems/:id/export` | 2 | Export problem as FPS |
| GET | `/api/contests/:id/plagiarism` | 5 | Run plagiarism check |
| GET | `/api/recommendations` | 5 | Get problem recommendations |
| POST | `/api/organizations` | 4 | Create organization |
| GET | `/api/organizations` | 4 | List organizations |
| GET | `/api/organizations/:slug` | 4 | Organization detail |
| POST | `/api/organizations/:slug/join` | 4 | Join organization |
| POST | `/api/organizations/:slug/classes` | 4 | Create class |
| POST | `/api/organizations/:slug/classes/:id/join` | 4 | Join class |
| GET | `/api/training` | 4 | List training plans |
| POST | `/api/training` | 4 | Create training plan |
| GET | `/api/training/:id` | 4 | Plan detail |
| POST | `/api/training/:id/enroll` | 4 | Enroll in plan |
| GET | `/api/training/:id/progress` | 4 | Get progress |
