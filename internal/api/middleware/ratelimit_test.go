package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRateLimit_KeysOnHostNotPort is the regression test for the throttling
// bypass: RemoteAddr includes an ephemeral port, so every proxied connection
// used to land in its own bucket.
func TestRateLimit_KeysOnHostNotPort(t *testing.T) {
	t.Setenv("RATE_LIMIT_RPS", "1")
	t.Setenv("RATE_LIMIT_BURST", "2")

	rl := NewRateLimiter()
	defer rl.Stop()

	h := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Same client, different source ports — all one bucket.
	for i, addr := range []string{"203.0.113.9:1000", "203.0.113.9:2000"} {
		req := httptest.NewRequest("GET", "/api/problems", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, rec.Code)
		}
	}

	req := httptest.NewRequest("GET", "/api/problems", nil)
	req.RemoteAddr = "203.0.113.9:59999" // new port, same client
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (a new port must not grant a new bucket)", rec.Code)
	}
}

func TestStrictAuth_KeysOnHostNotPort(t *testing.T) {
	t.Setenv("AUTH_RATE_LIMIT_RPS", "0.0001")
	t.Setenv("AUTH_RATE_LIMIT_BURST", "1")

	rl := NewRateLimiter()
	defer rl.Stop()

	h := rl.StrictAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRequest("POST", "/api/auth/login", nil)
	first.RemoteAddr = "198.51.100.4:1111"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, first)
	if rec.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec.Code)
	}

	second := httptest.NewRequest("POST", "/api/auth/login", nil)
	second.RemoteAddr = "198.51.100.4:2222" // same client, new port
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, second)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (login attempts must share one bucket)", rec.Code)
	}
}

// TestRateLimit_SeparatesClients guards against over-throttling: two real
// clients behind the same proxy must keep independent budgets.
func TestRateLimit_SeparatesClients(t *testing.T) {
	t.Setenv("RATE_LIMIT_RPS", "1")
	t.Setenv("RATE_LIMIT_BURST", "1")

	rl := NewRateLimiter()
	defer rl.Stop()

	trusted := ParseTrustedProxies("10.0.0.0/8")
	h := RealIP(trusted)(RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	send := func(clientIP string) int {
		req := httptest.NewRequest("GET", "/api/problems", nil)
		req.RemoteAddr = "10.0.0.2:443" // the proxy
		req.Header.Set("X-Forwarded-For", clientIP)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := send("203.0.113.1"); code != http.StatusOK {
		t.Fatalf("first client status = %d, want 200", code)
	}
	if code := send("203.0.113.1"); code != http.StatusTooManyRequests {
		t.Fatalf("first client second call = %d, want 429", code)
	}
	if code := send("203.0.113.2"); code != http.StatusOK {
		t.Fatalf("second client status = %d, want 200 (independent bucket)", code)
	}
}
