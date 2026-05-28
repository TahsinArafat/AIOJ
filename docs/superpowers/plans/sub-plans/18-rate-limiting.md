# Sub-Plan 18: Rate Limiting

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement rate limiting for API endpoints to prevent abuse.

**Architecture:** Add rate limiting middleware with Redis backend, per-user and per-endpoint limits.

**Tech Stack:** Go, Redis

---

## File Structure

### Backend Files to Create
- `internal/ratelimit/limiter.go` - Rate limiter implementation
- `internal/ratelimit/middleware.go` - Rate limit middleware
- `internal/ratelimit/store.go` - Redis store for rate limits

### Backend Files to Modify
- `internal/api/router.go` - Add rate limit middleware
- `docker-compose.yml` - Add Redis service

---

## Tasks

### Task 1: Add Redis to Docker Compose

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Add Redis service**

Add to `docker-compose.yml`:

```yaml
services:
  # ... existing services ...
  
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes

volumes:
  # ... existing volumes ...
  redis_data:
```

- [ ] **Step 2: Update .env.example**

Add:
```bash
REDIS_URL=redis://localhost:6379
```

- [ ] **Step 3: Commit**

```bash
git add docker-compose.yml .env.example
git commit -m "feat(ratelimit): add Redis to docker-compose"
```

---

### Task 2: Rate Limiter Implementation

**Files:**
- Create: `internal/ratelimit/limiter.go`
- Create: `internal/ratelimit/store.go`

- [ ] **Step 1: Create rate limiter**

```go
// internal/ratelimit/limiter.go
package ratelimit

import (
	"context"
	"time"
)

type Limiter struct {
	store  Store
	config Config
}

type Config struct {
	RequestsPerWindow int
	WindowDuration    time.Duration
}

type Result struct {
	Allowed    bool
	Remaining  int
	ResetAt    time.Time
	RetryAfter time.Duration
}

func NewLimiter(store Store, config Config) *Limiter {
	return &Limiter{
		store:  store,
		config: config,
	}
}

func (l *Limiter) Allow(ctx context.Context, key string) (*Result, error) {
	now := time.Now()
	windowStart := now.Truncate(l.config.WindowDuration)
	
	count, err := l.store.Increment(ctx, key, windowStart)
	if err != nil {
		return nil, err
	}
	
	remaining := l.config.RequestsPerWindow - count
	if remaining < 0 {
		remaining = 0
	}
	
	resetAt := windowStart.Add(l.config.WindowDuration)
	
	return &Result{
		Allowed:   count <= l.config.RequestsPerWindow,
		Remaining: remaining,
		ResetAt:   resetAt,
		RetryAfter: resetAt.Sub(now),
	}, nil
}

func (l *Limiter) GetUsage(ctx context.Context, key string) (int, error) {
	windowStart := time.Now().Truncate(l.config.WindowDuration)
	return l.store.Get(ctx, key, windowStart)
}
```

- [ ] **Step 2: Create Redis store**

```go
// internal/ratelimit/store.go
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store interface {
	Increment(ctx context.Context, key string, windowStart time.Time) (int, error)
	Get(ctx context.Context, key string, windowStart time.Time) (int, error)
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) Increment(ctx context.Context, key string, windowStart time.Time) (int, error) {
	windowKey := fmt.Sprintf("ratelimit:%s:%d", key, windowStart.Unix())
	
	pipe := s.client.Pipeline()
	incr := pipe.Incr(ctx, windowKey)
	pipe.Expire(ctx, windowKey, time.Hour)
	
	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}
	
	return int(incr.Val()), nil
}

func (s *RedisStore) Get(ctx context.Context, key string, windowStart time.Time) (int, error) {
	windowKey := fmt.Sprintf("ratelimit:%s:%d", key, windowStart.Unix())
	val, err := s.client.Get(ctx, windowKey).Int()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/ratelimit/
git commit -m "feat(ratelimit): add rate limiter implementation"
```

---

### Task 3: Rate Limit Middleware

**Files:**
- Create: `internal/ratelimit/middleware.go`

- [ ] **Step 1: Create middleware**

```go
// internal/ratelimit/middleware.go
package ratelimit

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Middleware struct {
	limiter *Limiter
	keyFunc func(r *http.Request) string
}

func NewMiddleware(limiter *Limiter, keyFunc func(r *http.Request) string) *Middleware {
	if keyFunc == nil {
		keyFunc = DefaultKeyFunc
	}
	return &Middleware{
		limiter: limiter,
		keyFunc: keyFunc,
	}
}

func DefaultKeyFunc(r *http.Request) string {
	// Use API key or IP address
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		return "apikey:" + apiKey
	}
	return "ip:" + r.RemoteAddr
}

func (m *Middleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := m.keyFunc(r)
		
		result, err := m.limiter.Allow(r.Context(), key)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		
		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(m.limiter.config.RequestsPerWindow))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))
		
		if !result.Allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 2: Add to router**

Update `internal/api/router.go`:

```go
// Create rate limiter
redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisURL})
rateLimitStore := ratelimit.NewRedisStore(redisClient)
rateLimiter := ratelimit.NewLimiter(rateLimitStore, ratelimit.Config{
	RequestsPerWindow: 100,
	WindowDuration:    time.Hour,
})
rateLimitMiddleware := ratelimit.NewMiddleware(rateLimiter, nil)

// Apply to API routes
r.Route("/api", func(r chi.Router) {
	r.Use(rateLimitMiddleware.Handle)
	// ... existing routes ...
})
```

- [ ] **Step 3: Commit**

```bash
git add internal/ratelimit/middleware.go internal/api/router.go
git commit -m "feat(ratelimit): add rate limit middleware"
```

---

## Verification Checklist

- [ ] Redis starts with docker-compose
- [ ] Rate limit headers returned
- [ ] 429 returned when limit exceeded
- [ ] Retry-After header set
- [ ] Per-user limits work
- [ ] Per-endpoint limits work

---

## Notes

1. **Window**: 1 hour (configurable)
2. **Default limit**: 100 requests per hour
3. **API keys**: Have custom limits
4. **Headers**: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset
