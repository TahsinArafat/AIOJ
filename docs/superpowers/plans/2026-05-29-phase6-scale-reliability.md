# Phase 6: Scale & Reliability — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make AIOJ production-ready for horizontal scale with Redis integration, distributed judge workers, database read replicas, and circuit breaker middleware.

**Architecture:** Redis as the backbone for caching, rate limiting, and message queuing. Judge worker extracted to separate process via `--mode=judge-worker`. Circuit breaker for graceful degradation. Read/Write split for PostgreSQL.

**Tech Stack:** Go 1.21+, PostgreSQL 18, Redis 7, Docker Compose

---

## Tasks

### Task 1: Redis Dependency & Config
**Files:** `go.mod`, `go.sum`, `internal/config/config.go`, `config.yaml`
- Add `github.com/redis/go-redis/v9` to go.mod
- Add `RedisConfig` struct to config
- Update config.yaml with redis section

### Task 2: Redis Client Wrapper
**Files:** Create `internal/cache/redis.go`, `internal/cache/redis_test.go`
- `NewRedisClient(cfg RedisConfig) *redis.Client` constructor
- `RedisCache` struct with Get/Set/Delete/Invalidate methods
- Falls back gracefully if Redis is unavailable

### Task 3: Redis Judge Queue
**Files:** Create `internal/queue/redis.go`, `internal/queue/redis_test.go`
- `RedisQueue` implementing `JudgeQueue` interface
- `Enqueue(ctx, id) error` - LPUSH
- `Dequeue(ctx) (string, error)` - BRPOP with 5s timeout

### Task 4: Distributed Judge Worker
**Files:** Modify `cmd/aioj/main.go`
- Add `--mode=judge-worker` flag
- In judge-worker mode: connect to Redis, start WorkerPool, don't start HTTP server

### Task 5: Rate Limiter Redis Store
**Files:** Modify `internal/api/middleware/ratelimit.go`
- Add `RateLimitStore` interface
- Implement `RedisRateLimitStore` with sliding window

### Task 6: Circuit Breaker Middleware
**Files:** Create `internal/middleware/circuit_breaker.go`, `internal/middleware/circuit_breaker_test.go`

### Task 7: Database Read Replicas
**Files:** Create `internal/store/postgres/readonly.go`

### Task 8: Docker Compose Updates
**Files:** Modify `docker-compose.yml`
- Add redis:7-alpine service
- Add judge-worker service with replicas

### Task 9: Fallback & Integration
**Files:** Modify `cmd/aioj/main.go`
- If REDIS_URL not set, fall back to MemoryQueue + no cache
