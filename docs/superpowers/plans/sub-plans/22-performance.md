# Sub-Plan 22: Performance

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Optimize application performance with caching, CDN, and query optimization.

**Architecture:** Add Redis caching layer, optimize database queries, add CDN headers.

**Tech Stack:** Go, Redis, PostgreSQL

---

## File Structure

### Backend Files to Create
- `internal/cache/cache.go` - Cache interface
- `internal/cache/redis.go` - Redis cache implementation
- `internal/cache/memory.go` - In-memory cache

### Backend Files to Modify
- `internal/store/postgres/problems.go` - Add caching
- `internal/store/postgres/contests.go` - Add caching
- `internal/api/handler/*.go` - Add cache headers

---

## Tasks

### Task 1: Cache Layer

**Files:**
- Create: `internal/cache/cache.go`
- Create: `internal/cache/redis.go`
- Create: `internal/cache/memory.go`

- [ ] **Step 1: Create cache interface**

```go
// internal/cache/cache.go
package cache

import (
	"context"
	"time"
)

type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	Clear(ctx context.Context, pattern string) error
}

type CacheConfig struct {
	DefaultTTL time.Duration
	Prefix     string
}
```

- [ ] **Step 2: Create Redis cache**

```go
// internal/cache/redis.go
package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
	config CacheConfig
}

func NewRedisCache(client *redis.Client, config CacheConfig) *RedisCache {
	return &RedisCache{
		client: client,
		config: config,
	}
}

func (c *RedisCache) Get(ctx context.Context, key string, dest interface{}) error {
	val, err := c.client.Get(ctx, c.config.Prefix+key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(val, dest)
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if ttl == 0 {
		ttl = c.config.DefaultTTL
	}
	return c.client.Set(ctx, c.config.Prefix+key, data, ttl).Err()
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.config.Prefix+key).Err()
}

func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	val, err := c.client.Exists(ctx, c.config.Prefix+key).Result()
	return val > 0, err
}

func (c *RedisCache) Clear(ctx context.Context, pattern string) error {
	keys, err := c.client.Keys(ctx, c.config.Prefix+pattern).Result()
	if err != nil {
		return err
	}
	if len(keys) > 0 {
		return c.client.Del(ctx, keys...).Err()
	}
	return nil
}
```

- [ ] **Step 3: Create in-memory cache**

```go
// internal/cache/memory.go
package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type MemoryCache struct {
	mu      sync.RWMutex
	items   map[string]cacheItem
	config  CacheConfig
}

type cacheItem struct {
	data      []byte
	expiresAt time.Time
}

func NewMemoryCache(config CacheConfig) *MemoryCache {
	cache := &MemoryCache{
		items:  make(map[string]cacheItem),
		config: config,
	}
	go cache.cleanup()
	return cache
}

func (c *MemoryCache) Get(ctx context.Context, key string, dest interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, exists := c.items[key]
	if !exists || time.Now().After(item.expiresAt) {
		return ErrCacheMiss
	}
	return json.Unmarshal(item.data, dest)
}

func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if ttl == 0 {
		ttl = c.config.DefaultTTL
	}
	
	c.mu.Lock()
	c.items[key] = cacheItem{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
	return nil
}

func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
	return nil
}

func (c *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, exists := c.items[key]
	return exists && time.Now().Before(item.expiresAt), nil
}

func (c *MemoryCache) Clear(ctx context.Context, pattern string) error {
	c.mu.Lock()
	for key := range c.items {
		delete(c.items, key)
	}
	c.mu.Unlock()
	return nil
}

func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.expiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

var ErrCacheMiss = fmt.Errorf("cache miss")
```

- [ ] **Step 4: Commit**

```bash
git add internal/cache/
git commit -m "feat(perf): add cache layer"
```

---

### Task 2: Cache Problem Queries

**Files:**
- Modify: `internal/store/postgres/problems.go`

- [ ] **Step 1: Add caching to problem queries**

```go
type CachedProblemStore struct {
	store *ProblemStore
	cache cache.Cache
}

func NewCachedProblemStore(store *ProblemStore, cache cache.Cache) *CachedProblemStore {
	return &CachedProblemStore{
		store: store,
		cache: cache,
	}
}

func (s *CachedProblemStore) GetBySlug(ctx context.Context, slug string) (*model.Problem, error) {
	key := fmt.Sprintf("problem:%s", slug)
	
	var problem model.Problem
	if err := s.cache.Get(ctx, key, &problem); err == nil {
		return &problem, nil
	}
	
	problemPtr, err := s.store.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	
	if problemPtr != nil {
		s.cache.Set(ctx, key, problemPtr, 10*time.Minute)
	}
	
	return problemPtr, nil
}

func (s *CachedProblemStore) List(ctx context.Context, offset, limit int) ([]model.ProblemListItem, int, error) {
	key := fmt.Sprintf("problems:list:%d:%d", offset, limit)
	
	type cachedList struct {
		Data  []model.ProblemListItem `json:"data"`
		Total int                     `json:"total"`
	}
	
	var cached cachedList
	if err := s.cache.Get(ctx, key, &cached); err == nil {
		return cached.Data, cached.Total, nil
	}
	
	items, total, err := s.store.List(ctx, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	
	s.cache.Set(ctx, key, cachedList{Data: items, Total: total}, 5*time.Minute)
	return items, total, nil
}

func (s *CachedProblemStore) Invalidate(ctx context.Context, slug string) {
	s.cache.Delete(ctx, fmt.Sprintf("problem:%s", slug))
	s.cache.Clear(ctx, "problems:list:*")
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/store/postgres/problems.go
git commit -m "feat(perf): add caching to problem queries"
```

---

### Task 3: HTTP Cache Headers

**Files:**
- Modify: `internal/api/handler/problem.go`
- Modify: `internal/api/handler/contest.go`

- [ ] **Step 1: Add cache headers to static responses**

```go
func (h *ProblemHandler) List(w http.ResponseWriter, r *http.Request) {
	// ... existing code ...
	
	// Add cache headers
	w.Header().Set("Cache-Control", "public, max-age=60") // 1 minute
	w.Header().Set("ETag", fmt.Sprintf("%x", md5.Sum(data)))
	
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *ProblemHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	// ... existing code ...
	
	// Add cache headers
	w.Header().Set("Cache-Control", "public, max-age=300") // 5 minutes
	
	respondJSON(w, http.StatusOK, problem)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/api/handler/problem.go internal/api/handler/contest.go
git commit -m "feat(perf): add HTTP cache headers"
```

---

### Task 4: Database Query Optimization

**Files:**
- Modify: `internal/store/migrations/000016_performance_indexes.up.sql`

- [ ] **Step 1: Create performance indexes**

```sql
-- internal/store/migrations/000016_performance_indexes.up.sql

-- Problem queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_problems_slug ON problems(slug);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_problems_difficulty ON problems(difficulty);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_problems_created ON problems(created_at DESC);

-- Submission queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_submissions_user_status ON submissions(user_id, status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_submissions_problem_user ON submissions(problem_id, user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_submissions_contest ON submissions(contest_id, created_at);

-- Contest queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_contests_type ON contests(type);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_contests_start ON contests(start_time DESC);

-- User queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_profiles_rating ON user_profiles(rating DESC);

-- Contest rankings
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_contest_ranks_contest ON contest_ranks(contest_id, total_score DESC);
```

- [ ] **Step 2: Run migration**

Run: `make migrate-up`

- [ ] **Step 3: Commit**

```bash
git add internal/store/migrations/000016_performance_indexes.*
git commit -m "feat(perf): add database performance indexes"
```

---

## Verification Checklist

- [ ] Cache hit rate > 80%
- [ ] Response time < 100ms for cached queries
- [ ] Cache invalidation works correctly
- [ ] HTTP cache headers present
- [ ] Database queries use indexes

---

## Notes

1. **Cache TTL**: Problems 10 min, Lists 5 min
2. **Invalidation**: On update/delete
3. **CDN**: Nginx handles static assets
4. **Indexes**: CONCURRENTLY to avoid locks
