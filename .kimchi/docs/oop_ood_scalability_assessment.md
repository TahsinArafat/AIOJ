# AIOJ Codebase Assessment: OOP, OOD, and Scalability

**Scope:** Backend Go codebase (`internal/`, `cmd/`)
**Date:** 2026-06-23
**Evidence:** Source inspection + `go test ./... -count=1` (343 passed in 36 packages)

---

## 1. Executive Summary

AIOJ is a monolithic Go online judge with a **reasonably clean layered architecture**. It demonstrates solid use of core object-oriented and design-pattern principles in its data-access layer, dependency injection, and queue/worker abstractions. However, several areas violate SOLID guidelines—most notably the `judge.WorkerPool`, which has grown into a god-object that compiles, executes, checks, and scores submissions across six different problem types. Scalability is partially addressed (Redis-backed distributed queue, separate `judge-worker` mode, Prometheus metrics), but database and worker scaling still have single-point-of-bottleneck risks.

| Dimension | Verdict | Confidence |
|-----------|---------|------------|
| OOP Principles | Mostly solid, with notable exceptions | High |
| OOD Design Patterns | Good use of common patterns; some anti-patterns | High |
| Scalability | Partially ready; needs work for true horizontal scale | Medium |

---

## 2. OOP Principles Assessment

### 2.1 Encapsulation — **Good**

- Handlers, stores, and services expose only the methods needed by callers.
- Fields are unexported in constructors (e.g., `SubmissionHandler` fields are lowercase).
- Database-specific SQL is hidden behind `store` interfaces.
- Configuration is centralized in `config.Config` and injected into components.

**Example:** `internal/store/interfaces.go` defines narrow interfaces such as `UserStore`, `ProblemStore`, etc., so callers cannot accidentally depend on PostgreSQL specifics.

### 2.2 Abstraction — **Good at the Store Layer, Mixed Elsewhere**

- The `store` package abstracts persistence cleanly. Any caller uses `store.SubmissionStore`, not `postgres.SubmissionStore`.
- The `queue.JudgeQueue` interface abstracts a priority queue, with in-memory and Redis implementations.
- The `vjudge.Bot` interface abstracts remote-OJ bots.

**Concern:** The abstraction leaks in `vjudge.Service.SetCookies` and `Submit`, which use type switches on concrete bot types (`*CodeforcesBot`, `*AtCoderBot`, etc.). This breaks abstraction and makes adding a new bot require modifying the service, not just registering a new implementation.

### 2.3 Inheritance — **Not Applicable / Avoided**

- Go does not support class-based inheritance. The codebase correctly uses composition and interfaces instead.
- `model.ProblemWithSamples` embeds `Problem`, which is idiomatic Go composition rather than inheritance.

### 2.4 Polymorphism — **Good, with Exceptions**

- `queue.JudgeQueue` is polymorphic: the same `WorkerPool` works with `MemoryQueue` or `RedisQueue`.
- `checker.GetChecker` returns different checker strategies behind a common interface.
- `vjudge.Bot` is polymorphic in principle, but the service-level type switches reduce the benefit.

### 2.5 Single Responsibility Principle (SRP) — **Mixed**

**Good:**
- `handler` files map roughly one-to-one with domain entities.
- `store` interfaces are narrow and entity-specific.
- `config.Config` is separated from business logic.

**Notable Violation:**
- `internal/judge/worker.go` (`WorkerPool`) is responsible for:
  - queue consumption,
  - compilation of contestant code,
  - compilation of SPJ and interactor code,
  - standard batch judging,
  - interactive judging,
  - output-only judging,
  - subtask scoring,
  - checker invocation,
  - balloon request creation,
  - problem submission-count updates.

This file is ~700 lines and mixes orchestration, compilation, execution, and scoring. Refactoring into smaller collaborators (`Compiler`, `Runner`, `Scorer`, `CheckerInvoker`, `InteractiveRunner`) would improve testability and SRP.

### 2.6 Open/Closed Principle — **Partial**

- Adding a new language is configuration-driven (`lang/*.yaml`) and does not require code changes — strong OCP.
- Adding a new store implementation satisfies OCP because callers depend on interfaces.
- Adding a new remote-OJ bot currently requires editing `vjudge.Service.Submit` and `SetCookies` due to type switches — violates OCP.
- Adding a new scoring/checker mode requires editing `WorkerPool` — violates OCP.

### 2.7 Liskov Substitution Principle — **Mostly Honored**

- `MemoryQueue` and `RedisQueue` are interchangeable via `queue.JudgeQueue`.
- `postgres.*Store` implementations can be swapped with mock implementations for tests.

**Concern:** `RedisQueue.Dequeue` returns `("", nil)` when the queue is empty, whereas `MemoryQueue` blocks until an item is available. Callers must handle both behaviors. This is a subtle LSP violation because substituting one queue for the other changes blocking semantics.

### 2.8 Interface Segregation Principle — **Good**

- `store/interfaces.go` defines many small, focused interfaces (`UserStore`, `ContestStore`, `HackStore`, etc.) rather than one giant data interface.
- Handlers receive only the stores they need (e.g., `SubmissionHandler` does not get a `BlogStore`).

### 2.9 Dependency Inversion Principle — **Good**

- High-level modules (handlers, services) depend on `store` interfaces, not concrete PostgreSQL types.
- `cmd/aioj/main.go` wires all dependencies manually (constructor injection), which is idiomatic Go and easy to follow.
- No global singleton stores or handlers were observed.

---

## 3. OOD Design Patterns Assessment

### 3.1 Repository / Data-Access-Object Pattern — **Implemented Well**

- `internal/store/interfaces.go` + `internal/store/postgres/*.go` form a classic repository layer.
- Each entity has its own repository interface and PostgreSQL implementation.
- This pattern is the strongest design element in the backend.

### 3.2 Dependency Injection (Manual) — **Implemented Well**

- `cmd/aioj/main.go` constructs stores, services, and handlers and injects them via constructors.
- `api.Deps` aggregates all handlers for the router.
- This makes the code testable and avoids global state.

### 3.3 Worker Pool / Producer-Consumer — **Implemented, but Monolithic**

- `judge.WorkerPool` consumes submission IDs from `queue.JudgeQueue` and dispatches goroutines bounded by a semaphore (`sem chan struct{}`).
- This is a valid worker-pool / producer-consumer pattern.
- The pool recovers from panics and marks submissions as System Error — a good reliability practice.

**Concern:** The worker itself is a monolith. The pattern is present, but its internal organization does not scale in complexity.

### 3.4 Strategy Pattern — **Present in Checkers, Leaky in VJudge**

- `checker.GetChecker(prob.CheckerType, prob.FloatEpsilon)` selects the appropriate output-checking strategy at runtime.
- This is a clean Strategy pattern.

**Concern:** `vjudge.Service` does not fully use Strategy for bot-specific cookie/configuration handling. Instead it uses type switches, which is a procedural alternative to Strategy.

### 3.5 Factory Pattern — **Configuration-Driven, Not Code Factories**

- Language compilers and runtime are loaded from YAML (`lang/*.yaml`) via `compiler.LoadLanguages`.
- This is a declarative factory approach and works well for the problem domain.
- No explicit Go factory functions were needed for most objects.

### 3.6 Registry Pattern — **Used in VJudge**

- `vjudge.Service` maintains a `map[string]Bot` registry.
- Bots are registered at startup in `main.go`.
- This is a reasonable use of Registry, but as noted above it is undermined by type switches in the service.

### 3.7 Adapter Pattern — **Implied**

- `executor.Client` adapts the external `go-judge` HTTP API to the project's `Cmd` / `ExecRequest` types.
- The `vjudge` bots adapt various remote OJ APIs (Codeforces, AtCoder, CSES, Toph, QOJ) to a common `Bot` interface.

### 3.8 Circuit Breaker — **Claimed but Not Verified**

- The project roadmap mentions "circuit breaker" as part of "Superiority Phase 6".
- During this review, no circuit-breaker implementation was found in the backend code for external calls (e.g., `go-judge`, `cf-submit`).
- Bot accounts do track `ConsecutiveFailures` and are skipped after 3 failures, which is a simple form of circuit breaking.

### 3.8 Singleton — **Avoided**

- No global singletons for stores or services were found.
- `slog` uses the default logger, which is acceptable for logging.

### 3.9 Anti-Patterns Observed

- **God Object:** `judge.WorkerPool`.
- **Type Switching Instead of Polymorphism:** `vjudge.Service.SetCookies`, `Submit`, `FetchLanguages`.
- **Primitive Obsession / Stringly Typed:** Some domain concepts use raw strings instead of types (e.g., `model.SubmissionStatus` is likely a string; roles like `"manager"`, `"judge"`, `"tester"` are passed as string literals).

---

## 4. Scalability Assessment

### 4.1 Compute Scaling — **Partially Ready**

**Strengths:**
- The `judge-worker` can run as a separate process (`mode == "judge-worker"`), decoupling judging from the API server.
- `queue.JudgeQueue` has a Redis-backed implementation, allowing multiple `judge-worker` instances to consume from a shared queue.
- Worker concurrency is configurable via `JUDGE_CONCURRENCY`.

**Concerns:**
- The in-memory `MemoryQueue` is the default when Redis is not configured. It cannot be shared across processes and is lost on restart.
- There is no coordination to prevent multiple workers from judging the same submission concurrently. The Redis queue uses `BZPOPMIN`, which is atomic, so this is likely safe, but the in-memory queue has no such guarantee under multiple worker processes.
- No horizontal auto-scaling logic is present; scaling is operational (run more containers) rather than automatic.

### 4.2 Database Scaling — **Limited**

**Strengths:**
- `config.Config.Database` includes `MaxOpen` and `MaxIdle` connection pool settings.
- The roadmap mentions "read replicas" as completed.

**Concerns:**
- No evidence of separate read/write database clients or replica routing in `internal/store/postgres`.
- No database-level sharding or partitioning strategy.
- Many list endpoints use `OFFSET/LIMIT` pagination, which becomes slow on very large tables.
- Hot tables such as `submissions` will grow quickly and could become a bottleneck without partitioning or archival.

### 4.3 Caching — **Minimal**

- Redis is used only for the judge queue.
- No caching layer for frequently read data (problems, contests, scoreboards, user stats).
- Scoreboards and statistics are computed on demand, which will not scale under heavy contest load.

### 4.4 Concurrency and Isolation — **Reasonable**

- `WorkerPool` uses a semaphore to bound concurrent judging.
- Panic recovery prevents a single bad submission from crashing the worker.
- `MemoryQueue` uses a `sync.Cond` for safe producer-consumer behavior.
- `vjudge.Service` uses `sync.RWMutex` for bot registry and `sync.Map` for active submissions.

**Concern:** Database stores may become a concurrency bottleneck if many workers update the same problem's `submission_count` / `accepted_count` simultaneously. There was no evidence of row-level locking or atomic counter updates.

### 4.5 Rate Limiting — **Present**

- `middleware.NewRateLimiter()` is applied globally in `api/router.go`.
- This is a positive scalability/reliability feature, though the exact algorithm (token bucket, fixed window, etc.) was not inspected.

### 4.6 Observability — **Present**

- Prometheus metrics endpoint (`/metrics`) is wired.
- Structured logging with `slog` is used consistently.
- Grafana dashboards and alerts are included in `deploy/grafana/`.

### 4.7 External Service Resilience — **Weak**

- Calls to `go-judge` (`executor.Client`) use a 30-second HTTP timeout.
- Calls to remote OJs (VJudge bots) have per-bot timeouts.
- No retry, backoff, or circuit-breaker logic was found for `go-judge` or remote OJ APIs beyond the simple bot-failure counter.
- `cf-submit` is a single external dependency; if it fails, Codeforces submissions fail with no fallback.

### 4.8 Frontend / API Scaling — **Not Assessed in Depth**

- The backend is a monolith; vertical scaling plus multiple containers behind a load balancer is the natural path.
- WebSocket manager (`handler.WSManager`) state is in-memory, which will prevent horizontal scaling of the API server unless sticky sessions or a shared pub/sub layer is added.

---

## 5. Strengths

1. **Interface-driven data layer** makes testing and swapping stores straightforward.
2. **Manual dependency injection** in `main.go` is clear and testable.
3. **Configuration-driven languages** satisfy Open/Closed for compiler/runtime definitions.
4. **Queue abstraction** allows local development (memory) and production scaling (Redis) with the same code.
5. **Separate `judge-worker` mode** enables distributed judging.
6. **Panic recovery in workers** improves reliability.
7. **Prometheus + structured logging** provide production observability.
8. **Test coverage is healthy:** 343 tests pass across 36 packages.

---

## 6. Weaknesses and Risks

1. **`judge.WorkerPool` is a god object** — violates SRP and OCP, and is hard to unit test in isolation.
2. **VJudge service uses type switches** — adding a new OJ requires editing the service, not just implementing an interface.
3. **Queue semantics differ** between memory and Redis implementations (blocking vs. non-blocking), risking subtle bugs when switching modes.
4. **No caching layer** for hot reads (problems, scoreboards, stats).
5. **WebSocket state is in-memory**, limiting horizontal scaling of API servers.
6. **Limited database scaling** — no read/write split, no partitioning, no archival strategy for submissions.
7. **No retry/backoff/circuit breaker** for `go-judge` or remote OJ APIs.
8. **Stringly typed domain values** (roles, statuses) reduce type safety and refactoring confidence.

---

## 7. Recommendations

### High Priority

1. **Refactor `judge.WorkerPool`** into smaller, single-responsibility collaborators:
   - `Compiler` — compile contestant/SPJ/interactor code.
   - `TestRunner` — run a single test case in the sandbox.
   - `Scorer` — aggregate results and compute final status/score.
   - `InteractiveRunner` — handle interactive problem protocol.
   - `OutputOnlyRunner` — handle output-only problems.
   - `WorkerPool` should only orchestrate dequeue → dispatch → finalize.

2. **Remove type switches in `vjudge.Service`** by extending the `Bot` interface with methods such as `Configure(cfg BotConfig)`, `SetCookies(cookies map[string]string)`, and `FetchLanguages(ctx context.Context)`. This lets new bots be added without touching the service.

3. **Unify queue semantics.** Decide whether `Dequeue` should block or return immediately, and make both implementations behave identically (or document and handle the difference explicitly).

### Medium Priority

4. **Add a caching layer** (Redis) for:
   - Problem lists and details.
   - Active contest scoreboards.
   - User statistics.

5. **Make WebSocket scaling horizontal** by moving WebSocket pub/sub to Redis or another message broker.

6. **Improve database scaling readiness:**
   - Consider table partitioning for `submissions` by time or contest.
   - Add read-replica routing if read load grows.
   - Replace `OFFSET/LIMIT` with keyset pagination for large lists.

### Low Priority / Nice to Have

7. **Introduce typed enums** for `SubmissionStatus`, roles, and access levels instead of raw strings.
8. **Add retry with exponential backoff and circuit breaking** for external HTTP dependencies (`go-judge`, remote OJs).
9. **Consider moving from monolith to modular monolith or bounded contexts** as the feature set continues to grow.

---

## 8. Conclusion

AIOJ is a **well-structured Go monolith for its scope**. It follows OOP/OOD best practices in its data layer, dependency injection, and queue abstractions, and it has taken meaningful steps toward scalability (Redis queue, separate judge workers, observability). The biggest risks are **concentrated complexity in the judging engine** and **incomplete abstraction in the VJudge service**. Addressing these two areas would significantly improve maintainability, testability, and the ability to add new problem types or remote OJs without destabilizing the codebase.

Scalability is sufficient for small-to-medium deployments but would require caching, database partitioning, and horizontal-safe WebSocket state before supporting large public contests with thousands of concurrent users.
