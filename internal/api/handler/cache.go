package handler

import (
	"sync"
	"time"
)

// TTLCache is a single-value cache with a time-to-live.  It is safe for
// concurrent use.  The zero value is ready to use with a zero TTL (every call
// refreshes), so callers must set TTL before first use.
//
// It is intentionally simple: one cached value, one loader function, no
// generics shenanigans that would require a Go version bump.  The loader is
// called under the write lock so only one goroutine refreshes at a time; all
// others block briefly then read the fresh value.
type TTLCache[T any] struct {
	mu      sync.RWMutex
	val     T
	loadedAt time.Time
	TTL     time.Duration
}

// Get returns the cached value, calling loader to refresh it when the cache is
// empty or stale.  If loader returns an error the stale value (if any) is
// returned together with the error so callers can decide whether to serve it.
func (c *TTLCache[T]) Get(loader func() (T, error)) (T, error) {
	// Fast path: value is fresh.
	c.mu.RLock()
	if !c.loadedAt.IsZero() && time.Since(c.loadedAt) < c.TTL {
		v := c.val
		c.mu.RUnlock()
		return v, nil
	}
	c.mu.RUnlock()

	// Slow path: refresh under write lock.
	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check: another goroutine may have refreshed while we waited.
	if !c.loadedAt.IsZero() && time.Since(c.loadedAt) < c.TTL {
		return c.val, nil
	}
	fresh, err := loader()
	if err != nil {
		// Return the stale value (if we have one) so a transient DB hiccup
		// doesn't serve a 500 to every crawler for the full TTL.
		return c.val, err
	}
	c.val = fresh
	c.loadedAt = time.Now()
	return c.val, nil
}
