package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// HealthChecker reports dependency status for /api/health.
type HealthChecker struct {
	DB *sql.DB
	// RedisPinger pings Redis when configured; nil skips Redis check.
	RedisPinger func(ctx context.Context) error
	// QueueDepth returns current judge queue length; nil skips.
	QueueDepth func() int
	// Started is process start time for uptime.
	Started time.Time
}

type healthComponent struct {
	Status string `json:"status"` // ok | degraded | down
	Error  string `json:"error,omitempty"`
	Value  any    `json:"value,omitempty"`
}

// Health: GET /api/health
// 200 when overall ok or degraded (still serving); 503 only if database is down.
func (h *HealthChecker) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	status := "ok"
	components := map[string]healthComponent{}

	// Database
	if h.DB != nil {
		if err := h.DB.PingContext(ctx); err != nil {
			components["database"] = healthComponent{Status: "down", Error: err.Error()}
			status = "down"
		} else {
			components["database"] = healthComponent{Status: "ok"}
		}
	} else {
		components["database"] = healthComponent{Status: "degraded", Error: "not configured"}
		if status == "ok" {
			status = "degraded"
		}
	}

	// Redis (optional)
	if h.RedisPinger != nil {
		if err := h.RedisPinger(ctx); err != nil {
			components["redis"] = healthComponent{Status: "degraded", Error: err.Error()}
			if status == "ok" {
				status = "degraded"
			}
		} else {
			components["redis"] = healthComponent{Status: "ok"}
		}
	}

	// Judge queue depth
	if h.QueueDepth != nil {
		depth := h.QueueDepth()
		components["judge_queue"] = healthComponent{Status: "ok", Value: depth}
	}

	uptime := 0.0
	if !h.Started.IsZero() {
		uptime = time.Since(h.Started).Seconds()
	}

	body := map[string]any{
		"status":     status,
		"components": components,
		"uptime_sec": uptime,
	}
	code := http.StatusOK
	if status == "down" {
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
