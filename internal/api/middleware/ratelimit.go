package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// visitor holds a rate limiter and last seen time for an IP.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter manages per-IP rate limiters using a token bucket algorithm.
type RateLimiter struct {
	visitors sync.Map // map[string]*visitor
	rps      rate.Limit
	burst    int
	stop     chan struct{}
}

// NewRateLimiter creates a RateLimiter configured from environment variables.
// RATE_LIMIT_RPS  — requests per second (default 100)
// RATE_LIMIT_BURST — burst size (default 200)
func NewRateLimiter() *RateLimiter {
	rps := 100.0
	if v := os.Getenv("RATE_LIMIT_RPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			rps = f
		}
	}

	burst := 200
	if v := os.Getenv("RATE_LIMIT_BURST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			burst = n
		}
	}

	rl := &RateLimiter{
		rps:   rate.Limit(rps),
		burst: burst,
		stop:  make(chan struct{}),
	}

	go rl.cleanup()
	return rl
}

// getVisitor returns the rate limiter for the given IP, creating one if needed.
func (rl *RateLimiter) getVisitor(ip string) *rate.Limiter {
	if v, ok := rl.visitors.Load(ip); ok {
		vis := v.(*visitor)
		vis.lastSeen = time.Now()
		return vis.limiter
	}

	limiter := rate.NewLimiter(rl.rps, rl.burst)
	rl.visitors.Store(ip, &visitor{limiter: limiter, lastSeen: time.Now()})
	return limiter
}

// cleanup removes stale entries every 10 minutes.
// Entries older than 1 hour are deleted.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.visitors.Range(func(key, value any) bool {
				vis := value.(*visitor)
				if time.Since(vis.lastSeen) > time.Hour {
					rl.visitors.Delete(key)
				}
				return true
			})
		case <-rl.stop:
			return
		}
	}
}

// Stop terminates the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stop)
}

// RateLimit returns chi-compatible middleware that enforces per-IP rate limits.
// Paths listed in skipPaths bypass rate limiting entirely.
func RateLimit(rl *RateLimiter, skipPaths ...string) func(http.Handler) http.Handler {
	skip := make(map[string]struct{}, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := skip[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}

			ip := r.RemoteAddr

			limiter := rl.getVisitor(ip)
			if !limiter.Allow() {
				reservation := limiter.Reserve()
				delay := reservation.Delay()
				reservation.Cancel()

				retryAfter := int(delay.Seconds()) + 1

				slog.Warn("rate limit exceeded",
					"ip", ip,
					"method", r.Method,
					"path", r.URL.Path,
					"retry_after", retryAfter,
				)

				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)

				json.NewEncoder(w).Encode(map[string]any{
					"error":       "rate limit exceeded",
					"retry_after": retryAfter,
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
