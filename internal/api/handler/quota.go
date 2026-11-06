package handler

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// userQuota is a per-user token bucket with idle eviction.
//
// The per-IP limiter cannot do this job: a classroom or a contest hall shares
// one public address, so a per-IP submission limit would either be useless for
// a single abusive user behind NAT or punish everyone behind it.
type userQuota struct {
	mu      sync.Mutex
	buckets map[string]*quotaBucket
	rps     rate.Limit
	burst   int
	// name is used in log lines so two quotas are distinguishable.
	name string
}

type quotaBucket struct {
	limiter *rate.Limiter
	last    time.Time
}

// maxTrackedUsers bounds the map. Beyond it, a sweep drops idle buckets; if
// everything is still active the oldest entries are evicted so a flood of
// distinct user IDs cannot exhaust memory.
const maxTrackedUsers = 8192

// idleQuotaEviction is how long a bucket may sit unused before it is dropped.
const idleQuotaEviction = 10 * time.Minute

func newUserQuota(name string, rps float64, burst int) *userQuota {
	if rps <= 0 {
		rps = 1
	}
	if burst <= 0 {
		burst = 1
	}
	return &userQuota{
		buckets: make(map[string]*quotaBucket),
		rps:     rate.Limit(rps),
		burst:   burst,
		name:    name,
	}
}

// allow consumes one token for userID. When the budget is exhausted it returns
// false and how long the caller should wait before retrying.
func (q *userQuota) allow(userID string) (bool, time.Duration) {
	now := time.Now()

	q.mu.Lock()
	b, ok := q.buckets[userID]
	if !ok {
		if len(q.buckets) >= maxTrackedUsers {
			q.sweepLocked(now)
		}
		b = &quotaBucket{limiter: rate.NewLimiter(q.rps, q.burst)}
		q.buckets[userID] = b
	}
	b.last = now
	limiter := b.limiter
	q.mu.Unlock()

	if limiter.AllowN(now, 1) {
		return true, 0
	}
	// Report the real wait rather than a fixed guess.
	reservation := limiter.ReserveN(now, 1)
	delay := reservation.DelayFrom(now)
	reservation.CancelAt(now)
	if delay < time.Second {
		delay = time.Second
	}
	return false, delay
}

// sweepLocked drops idle buckets. Callers must hold q.mu.
func (q *userQuota) sweepLocked(now time.Time) {
	for id, b := range q.buckets {
		if now.Sub(b.last) > idleQuotaEviction {
			delete(q.buckets, id)
		}
	}
	if len(q.buckets) < maxTrackedUsers {
		return
	}
	// Still full: evict the least recently used entry to keep the map bounded.
	var oldestID string
	var oldest time.Time
	for id, b := range q.buckets {
		if oldestID == "" || b.last.Before(oldest) {
			oldestID, oldest = id, b.last
		}
	}
	if oldestID != "" {
		delete(q.buckets, oldestID)
	}
}

func (q *userQuota) tracked() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.buckets)
}

// writeQuotaExceeded reports a 429 with the wait time in both the header and the
// body, so the SPA can show a countdown instead of a bare error.
func writeQuotaExceeded(w http.ResponseWriter, retryAfter time.Duration) {
	secs := int(retryAfter.Seconds())
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = fmt.Fprintf(w, `{"error":"too many requests","retry_after":%d}`, secs)
}

// envFloat/envInt read a numeric setting with a fallback.
func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return f
		}
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// Submission quotas. Defaults: one submission every 2s with a burst of 5, and
// a much tighter budget for /run, which compiles and executes arbitrary code on
// demand and is the cheapest way to occupy every judge worker.
//
// SUBMIT_RATE_LIMIT_RPS / SUBMIT_RATE_LIMIT_BURST
// RUN_RATE_LIMIT_RPS    / RUN_RATE_LIMIT_BURST
func newSubmissionQuota() *userQuota {
	return newUserQuota("submit",
		envFloat("SUBMIT_RATE_LIMIT_RPS", 0.5),
		envInt("SUBMIT_RATE_LIMIT_BURST", 5))
}

func newRunQuota() *userQuota {
	return newUserQuota("run",
		envFloat("RUN_RATE_LIMIT_RPS", 0.2),
		envInt("RUN_RATE_LIMIT_BURST", 3))
}
