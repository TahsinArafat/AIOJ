package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealth_NoDBDegraded(t *testing.T) {
	h := &HealthChecker{Started: time.Now().Add(-time.Second)}
	rr := httptest.NewRecorder()
	h.Health(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"status":"degraded"`) {
		t.Fatalf("body = %s", rr.Body.String())
	}
}

func TestHealth_QueueDepthReported(t *testing.T) {
	h := &HealthChecker{
		QueueDepth: func() int { return 7 },
		Started:    time.Now(),
	}
	rr := httptest.NewRecorder()
	h.Health(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"judge_queue"`) || !strings.Contains(body, `7`) {
		t.Fatalf("body = %s", body)
	}
}

func TestHealth_RedisDownDegrades(t *testing.T) {
	h := &HealthChecker{
		RedisPinger: func(context.Context) error { return errors.New("redis down") },
		QueueDepth:  func() int { return 0 },
	}
	rr := httptest.NewRecorder()
	h.Health(rr, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("code = %d want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"degraded"`) {
		t.Fatalf("body = %s", rr.Body.String())
	}
}

// ensures sql.DB nil path doesn't panic (covered above); compile-time check
var _ = sql.ErrNoRows
