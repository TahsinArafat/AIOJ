package middleware

import (
	"net/http"
	"sync/atomic"
	"time"
)

// CircuitBreaker returns a middleware that opens after consecutive failures.
func CircuitBreaker(name string, threshold int, timeout time.Duration) func(http.Handler) http.Handler {
	var failures int32
	var lastFailure time.Time

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.LoadInt32(&failures) >= int32(threshold) {
				if time.Since(lastFailure) < timeout {
					http.Error(w, "service temporarily unavailable", http.StatusServiceUnavailable)
					return
				}
				atomic.StoreInt32(&failures, 0)
			}

			next.ServeHTTP(w, r)
		})
	}
}
