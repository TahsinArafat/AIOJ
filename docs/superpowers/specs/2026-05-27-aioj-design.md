# AIOJ — Lightweight Online Judge Design Spec

> **Status:** Draft  
> **Last updated:** 2026-05-27  
> **Author:** AIOJ Team

**Goal:** Build a lightweight, efficient, scalable online judge for competitive programming — supporting problem solving, contests, and vJudge-style bot integrations.

**Architecture:** Unified Go monolith with clean module boundaries designed for future extraction. Single binary (API + judge worker in one process) communicating with PostgreSQL and a go-judge sidecar sandbox. React SPA frontend.

**Tech Stack:** Go 1.26, PostgreSQL 18, criyle/go-judge (sandbox), React + Vite + TypeScript + shadcn/ui. Docker Compose deployment.

---

## 1. Project Structure

```
AIOJ/
├── cmd/
│   └── aioj/                  # Main binary entrypoint
│       └── main.go
├── internal/
│   ├── api/                   # HTTP layer (Chi router)
│   │   ├── handler/           # Route handlers
│   │   ├── middleware/        # Auth, logging, rate limiting
│   │   └── router.go          # Route registration + middleware wiring
│   ├── judge/                 # Judging engine
│   │   ├── executor/          # go-judge HTTP client wrapper
│   │   ├── compiler/          # Compilation step within sandbox
│   │   ├── checker/           # Output comparison + SPJ support
│   │   └── worker.go          # Judge worker pool (goroutine semaphore)
│   ├── queue/                 # Submission queue interface
│   │   ├── interface.go       # JudgeQueue interface (Enqueue/Dequeue)
│   │   └── memory.go          # In-memory implementation via Go channels
│   ├── model/                 # Domain types (shared across packages)
│   │   ├── submission.go
│   │   ├── problem.go
│   │   ├── contest.go
│   │   ├── user.go
│   │   └── vjudge.go
│   ├── store/                 # Data access layer
│   │   ├── postgres/          # PostgreSQL implementation
│   │   ├── migrations/        # golang-migrate SQL files
│   │   └── interfaces.go      # Store interfaces (for test mocks)
│   ├── vjudge/                # Virtual judge / bot system
│   │   ├── bot.go             # Bot interface
│   │   ├── manager.go         # Bot lifecycle, rate limiting, scheduling
│   │   └── providers/         # Platform implementations
│   │       ├── codeforces.go
│   │       ├── atcoder.go
│   │       └── template.go    # Template for new providers
│   ├── auth/                  # Authentication & authorization
│   │   ├── jwt.go
│   │   ├── password.go        # bcrypt hashing
│   │   └── middleware.go
│   └── config/                # Configuration
│       └── config.go          # Viper-based config loading
├── lang/                      # Language definitions (YAML)
│   ├── c-gcc-32.yaml
│   ├── c-gcc-64.yaml
│   ├── cpp-gpp-32.yaml
│   ├── cpp-gpp-64.yaml
│   ├── cpp-clang.yaml
│   ├── python.yaml
│   ├── pypy.yaml
│   ├── javascript.yaml
│   ├── nodejs.yaml
│   ├── rust.yaml
│   ├── java.yaml
│   └── csharp.yaml
├── web/                       # React SPA
│   ├── src/
│   │   ├── components/        # Reusable UI components
│   │   ├── pages/             # Route pages
│   │   ├── hooks/             # Custom React hooks
│   │   ├── lib/               # API client, utilities
│   │   └── App.tsx
│   ├── package.json
│   └── vite.config.ts
├── deploy/
│   ├── Dockerfile             # Multi-stage Go build
│   ├── docker-compose.yml     # aioj + postgres + go-judge
│   └── nginx/                 # Reverse proxy for frontend
├── config.yaml                # Runtime configuration
├── go.mod
└── Makefile
```

### Module boundary rules
- `internal/api/` → never imports `internal/judge/` directly. Communicates through `internal/queue/`.
- `internal/judge/` → fully isolated. Can be extracted to separate binary by swapping `queue.NewMemory()` for `queue.NewRedis()`.
- `internal/store/` → interface-driven. Handlers and judge worker depend on interfaces, not concrete DB implementations.
- Lang configs → pure YAML, not code. No recompilation needed to add a language.

---

## 2. Data Model

### 2.1 User & Auth

```
User
  id              UUID        PK
  username        VARCHAR(64) UNIQUE, NOT NULL
  email           VARCHAR(255) UNIQUE, NOT NULL
  password_hash   VARCHAR(255) NOT NULL (bcrypt)
  role            ENUM('admin','teacher','user','bot') NOT NULL DEFAULT 'user'
  is_bot          BOOLEAN     DEFAULT false
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()

UserProfile
  user_id         UUID        PK, FK → User(id)
  rating          INTEGER     DEFAULT 0 (Elo-MMR)
  problems_solved INTEGER     DEFAULT 0
  submissions     INTEGER     DEFAULT 0
  bio             TEXT
  avatar_url      VARCHAR(512)
```

### 2.2 Problem

```
Problem
  id                UUID          PK
  slug              VARCHAR(128)  UNIQUE, NOT NULL (e.g. "a-plus-b")
  title             VARCHAR(256)  NOT NULL
  description       TEXT          NOT NULL (markdown)
  input_format      TEXT          (markdown)
  output_format     TEXT          (markdown)
  hint              TEXT          (markdown, optional)
  sample_cases      JSONB         NOT NULL DEFAULT '[]'
                                  [{input, output, explanation}]
  time_limit        INTEGER       NOT NULL (ms, default 1000)
  memory_limit      INTEGER       NOT NULL (KB, default 262144)
  difficulty        ENUM('easy','medium','hard')
  tags              TEXT[]        (Postgres array)
  visible           BOOLEAN       DEFAULT false
  testdata_path     VARCHAR(512)  NOT NULL (filesystem path to test cases)
  testcase_score    JSONB         [{input_name, output_name, score}]
  spj               BOOLEAN       DEFAULT false (special judge)
  spj_language      VARCHAR(64)   (if spj)
  spj_source_code   TEXT          (if spj)
  spj_version       VARCHAR(64)   (hash for cache busting)
  submission_count  INTEGER       DEFAULT 0 (denormalized)
  accepted_count    INTEGER       DEFAULT 0 (denormalized)
  source            VARCHAR(64)   -- 'local', 'codeforces', 'atcoder', etc. (for VJudge)
  remote_id         VARCHAR(128)  -- remote problem ID (e.g. "CF-4A")
  created_by        UUID          FK → User(id)
  created_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW()
  updated_at        TIMESTAMPTZ   NOT NULL DEFAULT NOW()
```

### 2.3 Submission

```
Submission
  id              UUID          PK
  problem_id      UUID          FK → Problem(id), NOT NULL
  user_id         UUID          FK → User(id), NOT NULL
  contest_id      UUID          FK → Contest(id), NULLABLE
  language        VARCHAR(64)   NOT NULL (matches lang YAML key)
  source_code     TEXT          NOT NULL (source text, optionally compressed)
  code_size       INTEGER       (bytes)
  status          ENUM('pending','judging','ac','wa','tle','mle','re','ce','se')
                                NOT NULL DEFAULT 'pending'
                                -- ac=Accepted, wa=WrongAnswer, tle=TimeLimit
                                -- mle=MemoryLimit, re=RuntimeError, ce=CompileError
                                -- se=SystemError
  score           INTEGER       (0-100, for OI/partial scoring)
  time_used       INTEGER       (ms, peak across test cases)
  memory_used     INTEGER       (KB, peak across test cases)
  compile_output  TEXT          (compiler stderr, if CE)
  judge_result    JSONB         (detailed per-testcase results)
                                [{case_name, status, time, memory, score, detail}]
  judged_by       VARCHAR(128)  (judge worker ID)
  created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
  judged_at       TIMESTAMPTZ

  INDEX: (problem_id, status)             -- queue polling
  INDEX: (user_id, created_at DESC)       -- user history
  INDEX: (contest_id, problem_id, status) -- contest ranks
  INDEX: (created_at)                     -- recent activity
```

### 2.4 Contest

```
Contest
  id              UUID          PK
  title           VARCHAR(256)  NOT NULL
  type            ENUM('acm','oi','ioi','practice') NOT NULL DEFAULT 'acm'
  start_time      TIMESTAMPTZ   NOT NULL
  end_time        TIMESTAMPTZ   NOT NULL
  freeze_time     TIMESTAMPTZ   (scoreboard frozen from this time, nullable)
  password        VARCHAR(128)  (nullable, for private contests)
  visible         BOOLEAN       DEFAULT true
  description     TEXT          (markdown)
  created_by      UUID          FK → User(id)
  created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()

ContestProblem
  contest_id      UUID          FK → Contest(id)
  problem_id      UUID          FK → Problem(id)
  index           CHAR(1)       NOT NULL ('A', 'B', 'C', ...)
  score           INTEGER       (max score for OI problems, default 100)
  sort_order      INTEGER       DEFAULT 0
  PK: (contest_id, problem_id)

ContestSubmission — lightweight view linking submissions to contests
  submission_id   UUID          PK, FK → Submission(id)
  contest_id      UUID          FK → Contest(id)
  problem_index   CHAR(1)
  user_id         UUID          FK → User(id)

ContestRank
  contest_id      UUID          PK, FK → Contest(id)
  user_id         UUID          PK, FK → User(id)
  problems        JSONB         NOT NULL DEFAULT '{}'
                                { "A": {solved, attempts, time, score, ...} }
  total_solved    INTEGER       DEFAULT 0 (ACM)
  total_penalty   INTEGER       DEFAULT 0 (ACM)
  total_score     INTEGER       DEFAULT 0 (OI)
  last_ac_time    TIMESTAMPTZ   (last accepted submission time, for tiebreak)

ContestAnnouncement
  id              UUID          PK
  contest_id      UUID          FK → Contest(id)
  title           VARCHAR(256)
  content         TEXT
  created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
```

### 2.5 VJudge Bot Accounts

```
BotAccount
  id              UUID          PK
  user_id         UUID          FK → User(id), unique (one bot user per platform)
  platform        VARCHAR(64)   NOT NULL ('codeforces', 'atcoder', ...)
  platform_user   VARCHAR(128)  (encrypted)
  platform_pass   VARCHAR(256)  (encrypted, using AEAD)
  session_data    JSONB         (encrypted cookie/token cache)
  status          ENUM('active','expired','error','banned')
  rate_limit_rps  REAL          DEFAULT 1.0
  last_used_at    TIMESTAMPTZ
  created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
```

---

## 3. Judging Pipeline

### 3.1 Flow

```
1. POST /api/submissions  → handler validates (lang, problem, code ≤ 256KB)
2. INSERT Submission(status=pending)
3. queue.Enqueue(submissionID)
4. JudgeWorker dequeues → UPDATE status=judging
5. FOR each test case (parallel with goroutines):
   a. Compile via go-judge (returns .out binary or compile error)
   b. If compile error → status=ce, store compile_output
   c. Execute via go-judge with stdin=testcase.in, capture stdout/stderr
   d. Compare output via checker
6. Aggregate per-case results into final verdict
7. UPDATE Submission(status, score, time_used, memory_used, judge_result)
8. WebSocket push: notify user of result
```

### 3.2 go-judge integration

go-judge runs as a sidecar container at `localhost:5050`. Communication via HTTP.

```go
// internal/judge/executor/executor.go

// ExecRequest maps to go-judge's /run endpoint
type ExecRequest struct {
    Cmd         []Cmd           `json:"cmd"`
    PipeInput   bool            `json:"pipeInput,omitempty"`
}

type Cmd struct {
    Args        []string            `json:"args"`
    Env         []string            `json:"env,omitempty"`
    CPULimit    uint64              `json:"cpuLimit"`
    MemoryLimit uint64              `json:"memoryLimit"`
    ProcLimit   uint64              `json:"procLimit"`
    CopyIn      map[string]File     `json:"copyIn,omitempty"`
    CopyOut     []string            `json:"copyOut,omitempty"`
    CopyOutDir  string              `json:"copyOutDir,omitempty"`
}

type File struct {
    Content  string `json:"content,omitempty"`  // inline content
    Src      string `json:"src,omitempty"`      // path in container
}

type ExecResponse struct {
    Results []CmdResult `json:"results"`
}

type CmdResult struct {
    Status     string `json:"status"`     // "Accepted", "RuntimeError", etc.
    ExitStatus int    `json:"exitStatus"`
    Error      string `json:"error,omitempty"`
    Time       uint64 `json:"time"`       // ns
    Memory     uint64 `json:"memory"`     // bytes
    RunDir     string `json:"runDir"`
    Files      map[string]File   `json:"files,omitempty"`
}
```

### 3.3 Language config format (YAML)

```yaml
# lang/cpp-gpp-64.yaml
name: "C++ (g++ 17, 64-bit)"
key: "cpp-gpp-64"
compile:
  command: "/usr/bin/g++ -std=c++17 -O2 -o {{exe}} {{src}}"
  # files to copy into sandbox before compile
  copy_in:
    "/usr/include": "/usr/include:ro"  # read-only mount
time_limit_multiplier: 1.0        # Python gets 3x
memory_limit_multiplier: 1.0
seccomp_rule: "c_cpp"             # syscall allowlist name
extensions: [".cpp", ".cxx", ".cc"]
mono: false                       # needs Mono runtime?
runtime: ""                       # interpreter command (for script langs)
```

### 3.4 Checker types

```go
// internal/judge/checker/checker.go

type Checker interface {
    Name() string
    Check(input, expected, actual []byte) (bool, int, string)
    // returns (passed, score_percent, message)
}

// Built-in checkers
type ExactChecker struct{}        // byte-exact comparison (trailing whitespace trimmed)
type LinesChecker struct{}        // line-by-line comparison (order-independent)
type FloatChecker struct{}        // numeric tolerance comparison

// Special Judge (SPJ) — external binary
type SPJChecker struct {
    Path     string   // path to compiled SPJ binary
    Language string
    Version  string
}
```

### 3.5 Concurrency & worker pool

```go
// internal/judge/worker.go

type WorkerPool struct {
    queue     queue.JudgeQueue
    exec      *executor.Client   // go-judge HTTP client
    concurrency int              // max parallel judge jobs (default: CPU * 2)
    sem       chan struct{}      // semaphore
}

func NewWorkerPool(queue queue.JudgeQueue, exec *executor.Client, concurrency int) *WorkerPool {
    return &WorkerPool{
        queue:       queue,
        exec:        exec,
        concurrency: concurrency,
        sem:         make(chan struct{}, concurrency),
    }
}

func (wp *WorkerPool) Start(ctx context.Context) {
    for {
        subID, err := wp.queue.Dequeue(ctx)
        if err != nil { continue }

        wp.sem <- struct{}{}  // acquire
        go func(id uuid.UUID) {
            defer func() { <-wp.sem }()
            wp.judge(ctx, id)
        }(subID)
    }
}
```

### 3.6 Verdict priority (ACM mode)

```
CE > (any TLE) > (any MLE) > (any RE) > WA > AC
```

For OI mode, scores sum across all test cases. Partial scoring per test case.

---

## 4. API Design

### 4.1 REST endpoints

```
POST   /api/auth/register          — Register new user
POST   /api/auth/login             — Login, returns JWT
POST   /api/auth/refresh           — Refresh JWT

GET    /api/users/me               — Get current user profile
PATCH  /api/users/me               — Update profile
GET    /api/users/:id              — Get public profile

GET    /api/problems               — List problems (paginated, filterable)
GET    /api/problems/:slug         — Get problem detail
POST   /api/problems               — Create problem (admin/teacher)
PATCH  /api/problems/:slug         — Update problem (admin/teacher)

GET    /api/problems/:slug/submissions   — List submissions for a problem
POST   /api/submissions            — Submit code (source_code, language, problem_id, contest_id?)
GET    /api/submissions/:id        — Get submission detail (incl. judge result)
GET    /api/submissions/:id/output — Download test case output files

GET    /api/contests               — List contests
POST   /api/contests               — Create contest (admin)
GET    /api/contests/:id           — Contest detail
PATCH  /api/contests/:id           — Update contest (admin)
GET    /api/contests/:id/rank      — Scoreboard
GET    /api/contests/:id/submissions — Contest submissions (admin)
POST   /api/contests/:id/rejudge   — Rejudge all contest submissions (admin)

GET    /api/rankings               — Global user rankings
GET    /api/languages              — List available languages

WS     /api/ws/submissions/:id     — Real-time submission status push
```

### 4.2 WebSocket protocol

```
Client → Server:  {"type": "subscribe", "submission_id": "uuid"}
Server → Client:  {"type": "status", "submission_id": "uuid", "status": "judging"}
Server → Client:  {"type": "result", "submission_id": "uuid", "result": {...}}
Server → Client:  {"type": "scoreboard", "contest_id": "uuid", "data": {...}}
```

---

## 5. VJudge / Bot System

### 5.1 Bot interface

```go
// internal/vjudge/bot.go

type Bot interface {
    Name() string                                // e.g. "codeforces"
    Submit(ctx context.Context, req *SubmitRequest) (*SubmitResponse, error)
    Poll(ctx context.Context, token string) (*PollResponse, error)
    MaxConcurrency() int                         // per-bot rate limit
}

type SubmitRequest struct {
    ProblemID  string   // remote problem ID
    SourceCode string
    Language   string   // mapped to remote language
    BotID      uuid.UUID
}

type SubmitResponse struct {
    Token string   // opaque tracking token
}

type PollResponse struct {
    Status   Verdict
    Time     int     // ms
    Memory   int     // KB
    Output   string  // remote compile output or error
    Done     bool
}
```

### 5.2 Problem mapping

When a problem's `source` is not `"local"`, submission goes through the VJudge pipeline:
1. Look up the Bot for the platform (e.g. "codeforces")
2. Bot submits code to the remote OJ
3. Poll until result arrives (respecting rate limits)
4. Map remote verdict → AIOJ verdict
5. Store submission with VJudge metadata

### 5.3 Language mapping

```go
var languageMap = map[string]map[string]string{
    "codeforces": {
        "cpp-gpp-64": "54",   // CF language ID for G++ 20 64-bit
        "python":     "70",   // Python 3
        "rust":       "93",   // Rust 2021
    },
}
```

### 5.4 Rate limiting and queue

```go
// internal/vjudge/manager.go

type Manager struct {
    bots    map[string]Bot
    limiter *rateLimiter  // per-platform token bucket
    queue   chan VJob     // pending VJudge jobs
}
```

---

## 6. Contest & Scoreboard

### 6.1 ACM scoring
- Problems solved = total unique problems with an accepted submission
- Penalty = sum of (first AC time in minutes + 20 × (wrong attempts before AC))
- Wrong submissions on unsolved problems contribute no penalty
- Tiebreak: fewer penalty wins; if still tied, last AC time earlier wins

### 6.2 OI scoring
- Each problem has a max score (default 100)
- Each test case has a score weight
- Total = sum of per-test-case scores
- No time penalty

### 6.3 Frozen scoreboard
- At `freeze_time`, scoreboard stops updating
- Non-frozen participants see their own results
- After contest ends, unfreeze and finalize
- "?" displayed for submissions judged during freeze

### 6.4 Rejudge
Admin can trigger a rejudge for:
- A single submission
- All submissions for a problem
- All submissions in a contest

Rejudge sets status back to `pending`, enqueues, and overwrites old results.

---

## 7. Auth & Security

| Feature | Implementation |
|---------|---------------|
| Password hashing | bcrypt (cost=12) |
| JWT access tokens | 15 min expiry, HMAC-SHA256 |
| JWT refresh tokens | 7 day expiry, stored hashed in DB |
| API keys | For bot access, 256-bit random, SHA-256 stored |
| Rate limiting | Token bucket per IP/user, configurable |
| CORS | Restricted to frontend origin |
| Input validation | Strict: code size ≤ 256KB, language whitelist |
| SQL injection | Parameterized queries only (no string building) |

---

## 8. Deployment

### 8.1 docker-compose.yml (v1)

```yaml
version: "3.9"
services:
  postgres:
    image: postgres:18-alpine
    environment:
      POSTGRES_DB: aioj
      POSTGRES_USER: aioj
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  go-judge:
    image: criyle/go-judge:latest
    privileged: true    # required for cgroups/namespaces
    environment:
      - GIN_MODE=release
    ports:
      - "5050:5050"

  aioj:
    build: .
    depends_on:
      - postgres
      - go-judge
    environment:
      - DB_HOST=postgres
      - DB_PASSWORD=${DB_PASSWORD}
      - JUDGE_ENDPOINT=http://go-judge:5050
    ports:
      - "8080:8080"

  aioj-web:
    build: ./web
    ports:
      - "3000:80"

volumes:
  pgdata:
```

### 8.2 Dockerfile (Go backend)

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o aioj ./cmd/aioj

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /build/aioj /usr/local/bin/
COPY lang/ /app/lang/
COPY config.yaml /app/
WORKDIR /app
CMD ["aioj"]
```

### 8.3 Nginx (frontend)

```nginx
server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    location /api/ {
        proxy_pass http://aioj:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    location /ws/ {
        proxy_pass http://aioj:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

---

## 9. Development Plan (Phases)

| Phase | Scope | Effort |
|-------|-------|--------|
| **1. Foundation** | Project scaffolding, config, DB migrations, auth, user model | Week 1 |
| **2. Problem CRUD** | Problem model, create/list problems, test data upload, lang configs | Week 2 |
| **3. Judging engine** | go-judge integration, compile+run, checkers, worker pool | Week 2-3 |
| **4. Submission API** | Submit, view results, status polling, WS notifications | Week 3 |
| **5. Frontend** | Problem view, code editor, submission list, auth pages | Week 3-4 |
| **6. Contest system** | Contest CRUD, scoreboard, freeze, rejudge | Week 4-5 |
| **7. VJudge bots** | Bot interface, Codeforces provider, problem mapping | Week 5-6 |
| **8. Polish & scale** | Admin dashboard, rate limiting, monitoring, Redis queue | Week 6-7 |

---

## 10. Open Questions (Deferred)

1. **Test data upload** — Admin uploads ZIP of test cases via API or CLI? (API for v1)
2. **Plagiarism detection** — MOSS integration? (Deferred post-v1)
3. **Rating system** — Elo-MMR (like DMOJ)? (Deferred until user base grows)
4. **Problem groups** — Tag-based filtering sufficient for v1
5. **Internationalization** — English only for v1
