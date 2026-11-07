package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// visitor holds a rate limiter and last seen time for an IP.
// lastSeen is atomic: the request path writes it while the cleanup goroutine
// reads it, and a plain time.Time is a multi-word value that races.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen atomic.Int64 // unix nanoseconds
}

func newVisitor(l *rate.Limiter) *visitor {
	v := &visitor{limiter: l}
	v.touch()
	return v
}

func (v *visitor) touch() { v.lastSeen.Store(time.Now().UnixNano()) }

func (v *visitor) idleFor() time.Duration {
	return time.Since(time.Unix(0, v.lastSeen.Load()))
}

// RateLimiter manages per-IP rate limiters using a token bucket algorithm.
type RateLimiter struct {
	visitors sync.Map // map[string]*visitor
	rps      rate.Limit
	burst    int
	// Strict-auth budget. Defaults reproduce the previous hardcoded values
	// (1 req / 5s, burst 5); overridable so the Playwright suite — which makes
	// many auth calls from a single loopback IP — is not throttled in the sim.
	strictRPS   rate.Limit
	strictBurst int
	stopFn      context.CancelFunc
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

	// AUTH_RATE_LIMIT_RPS  — sustained auth requests/sec (default 0.2 = 1 per 5s)
	// AUTH_RATE_LIMIT_BURST — auth burst (default 5)
	strictRPS := 0.2
	if v := os.Getenv("AUTH_RATE_LIMIT_RPS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			strictRPS = f
		}
	}
	strictBurst := 5
	if v := os.Getenv("AUTH_RATE_LIMIT_BURST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			strictBurst = n
		}
	}

	rl := &RateLimiter{
		rps:         rate.Limit(rps),
		burst:       burst,
		strictRPS:   rate.Limit(strictRPS),
		strictBurst: strictBurst,
	}

	cleanupCtx, cancel := context.WithCancel(context.Background())
	rl.stopFn = cancel
	go rl.cleanup(cleanupCtx)
	return rl
}

// getVisitor returns the rate limiter for the given IP, creating one if needed.
func (rl *RateLimiter) getVisitor(ip string) *rate.Limiter {
	if v, ok := rl.visitors.Load(ip); ok {
		vis := v.(*visitor)
		vis.touch()
		return vis.limiter
	}

	limiter := rate.NewLimiter(rl.rps, rl.burst)
	actual, _ := rl.visitors.LoadOrStore(ip, newVisitor(limiter))
	vis := actual.(*visitor)
	vis.touch()
	return vis.limiter
}

// cleanup removes stale entries every 10 minutes.
// Entries older than 1 hour are deleted.
// The goroutine exits cleanly when ctx is cancelled (via Stop).
func (rl *RateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.visitors.Range(func(key, value any) bool {
				vis := value.(*visitor)
				if vis.idleFor() > time.Hour {
					rl.visitors.Delete(key)
				}
				return true
			})
		case <-ctx.Done():
			return
		}
	}
}

// Stop terminates the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	rl.stopFn()
}

// StrictAuth is a chi-compatible middleware enforcing a tight per-IP+path
// budget on auth endpoints (defaults: 5 burst, 1 req / 5s sustained).
func (rl *RateLimiter) StrictAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := "strict:" + ClientIP(r) + ":" + r.URL.Path
		limiter, _ := rl.visitors.LoadOrStore(key, newVisitor(
			rate.NewLimiter(rl.strictRPS, rl.strictBurst)))
		vis := limiter.(*visitor)
		vis.touch()
		if !vis.limiter.Allow() {
			w.Header().Set("Retry-After", "5")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
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

			// Host only: RemoteAddr carries an ephemeral port, so keying on it
			// gave every connection a fresh bucket and defeated the limit.
			ip := ClientIP(r)

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
