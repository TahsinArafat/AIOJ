# Sub-Plan 23: Monitoring

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add comprehensive monitoring with metrics, logging, and alerting.

**Architecture:** Add Prometheus metrics, structured logging, health checks, and Grafana dashboards.

**Tech Stack:** Go, Prometheus, Grafana, ELK Stack (optional)

---

## File Structure

### Backend Files to Create
- `internal/metrics/metrics.go` - Prometheus metrics
- `internal/logging/logger.go` - Structured logging
- `internal/health/health.go` - Health check endpoint

### Backend Files to Modify
- `internal/api/middleware/logging.go` - Add request metrics
- `cmd/aioj/main.go` - Initialize metrics
- `docker-compose.yml` - Add Prometheus and Grafana

---

## Tasks

### Task 1: Prometheus Metrics

**Files:**
- Create: `internal/metrics/metrics.go`

- [ ] **Step 1: Create metrics collector**

```go
// internal/metrics/metrics.go
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aioj_http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	
	HttpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aioj_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5},
		},
		[]string{"method", "path"},
	)
	
	// Submission metrics
	SubmissionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aioj_submissions_total",
			Help: "Total submissions",
		},
		[]string{"language", "status"},
	)
	
	SubmissionDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "aioj_submission_judge_duration_seconds",
			Help:    "Submission judge duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120},
		},
	)
	
	// Contest metrics
	ActiveContests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "aioj_active_contests",
			Help: "Number of active contests",
		},
	)
	
	ContestParticipants = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aioj_contest_participants",
			Help: "Number of contest participants",
		},
		[]string{"contest_id"},
	)
	
	// User metrics
	RegisteredUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "aioj_registered_users",
			Help: "Total registered users",
		},
	)
	
	OnlineUsers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "aioj_online_users",
			Help: "Currently online users",
		},
	)
	
	// System metrics
	DatabaseConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "aioj_database_connections",
			Help: "Active database connections",
		},
	)
	
	CacheHitRate = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "aioj_cache_hit_rate",
			Help: "Cache hit rate",
		},
	)
)
```

- [ ] **Step 2: Commit**

```bash
git add internal/metrics/
git commit -m "feat(monitoring): add Prometheus metrics"
```

---

### Task 2: Structured Logging

**Files:**
- Create: `internal/logging/logger.go`

- [ ] **Step 1: Create logger**

```go
// internal/logging/logger.go
package logging

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type Logger struct {
	*logrus.Logger
}

func NewLogger(level string) *Logger {
	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	
	switch level {
	case "debug":
		log.SetLevel(logrus.DebugLevel)
	case "info":
		log.SetLevel(logrus.InfoLevel)
	case "warn":
		log.SetLevel(logrus.WarnLevel)
	case "error":
		log.SetLevel(logrus.ErrorLevel)
	default:
		log.SetLevel(logrus.InfoLevel)
	}
	
	return &Logger{log}
}

func (l *Logger) WithRequest(method, path, ip string) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"method": method,
		"path":   path,
		"ip":     ip,
	})
}

func (l *Logger) WithUser(userID, username string) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"user_id":  userID,
		"username": username,
	})
}

func (l *Logger) WithSubmission(submissionID, problemID, language string) *logrus.Entry {
	return l.WithFields(logrus.Fields{
		"submission_id": submissionID,
		"problem_id":    problemID,
		"language":      language,
	})
}

func (l *Logger) WithError(err error) *logrus.Entry {
	return l.WithField("error", err.Error())
}
```

- [ ] **Step 2: Update middleware to use logger**

```go
// internal/api/middleware/logging.go
func Logging(logger *logging.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			wrapped := wrapResponseWriter(w)
			next.ServeHTTP(wrapped, r)
			
			duration := time.Since(start)
			
			logger.WithRequest(r.Method, r.URL.Path, r.RemoteAddr).
				WithFields(logrus.Fields{
					"status":   wrapped.status,
					"duration": duration.Milliseconds(),
				}).
				Info("request completed")
			
			// Update metrics
			metrics.HttpRequestsTotal.WithLabelValues(
				r.Method,
				r.URL.Path,
				strconv.Itoa(wrapped.status),
			).Inc()
			
			metrics.HttpRequestDuration.WithLabelValues(
				r.Method,
				r.URL.Path,
			).Observe(duration.Seconds())
		})
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/logging/ internal/api/middleware/logging.go
git commit -m "feat(monitoring): add structured logging"
```

---

### Task 3: Health Check Endpoint

**Files:**
- Create: `internal/health/health.go`

- [ ] **Step 1: Create health checker**

```go
// internal/health/health.go
package health

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type HealthChecker struct {
	db    *sql.DB
	redis *redis.Client
}

func NewHealthChecker(db *sql.DB, redis *redis.Client) *HealthChecker {
	return &HealthChecker{
		db:    db,
		redis: redis,
	}
}

type HealthStatus struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"`
}

func (h *HealthChecker) Check(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	
	status := HealthStatus{
		Status:   "healthy",
		Services: make(map[string]string),
	}
	
	// Check database
	if err := h.db.PingContext(ctx); err != nil {
		status.Status = "unhealthy"
		status.Services["database"] = "unhealthy: " + err.Error()
	} else {
		status.Services["database"] = "healthy"
	}
	
	// Check Redis
	if h.redis != nil {
		if err := h.redis.Ping(ctx).Err(); err != nil {
			status.Status = "unhealthy"
			status.Services["redis"] = "unhealthy: " + err.Error()
		} else {
			status.Services["redis"] = "healthy"
		}
	}
	
	httpStatus := http.StatusOK
	if status.Status == "unhealthy" {
		httpStatus = http.StatusServiceUnavailable
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(status)
}
```

- [ ] **Step 2: Add health endpoint to router**

```go
// Add to router
r.Get("/health", healthChecker.Check)
r.Get("/ready", healthChecker.Check)
```

- [ ] **Step 3: Commit**

```bash
git add internal/health/ internal/api/router.go
git commit -m "feat(monitoring): add health check endpoints"
```

---

### Task 4: Docker Monitoring Stack

**Files:**
- Modify: `docker-compose.yml`
- Create: `deploy/prometheus/prometheus.yml`
- Create: `deploy/grafana/dashboards/aioj.json`

- [ ] **Step 1: Add Prometheus to docker-compose**

Add to `docker-compose.yml`:

```yaml
services:
  # ... existing services ...
  
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./deploy/prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
    restart: unless-stopped

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    volumes:
      - grafana_data:/var/lib/grafana
      - ./deploy/grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./deploy/grafana/datasources:/etc/grafana/provisioning/datasources
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    restart: unless-stopped

volumes:
  # ... existing volumes ...
  prometheus_data:
  grafana_data:
```

- [ ] **Step 2: Create Prometheus config**

```yaml
# deploy/prometheus/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'aioj'
    static_configs:
      - targets: ['backend:8080']
    metrics_path: '/metrics'
```

- [ ] **Step 3: Create Grafana datasource**

```yaml
# deploy/grafana/datasources/prometheus.yml
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
```

- [ ] **Step 4: Create Grafana dashboard**

```json
// deploy/grafana/dashboards/aioj.json
{
  "dashboard": {
    "title": "AIOJ Dashboard",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(aioj_http_requests_total[5m])",
            "legendFormat": "{{method}} {{path}}"
          }
        ]
      },
      {
        "title": "Response Time",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(aioj_http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "p95"
          }
        ]
      },
      {
        "title": "Active Contests",
        "type": "stat",
        "targets": [
          {
            "expr": "aioj_active_contests"
          }
        ]
      },
      {
        "title": "Online Users",
        "type": "stat",
        "targets": [
          {
            "expr": "aioj_online_users"
          }
        ]
      }
    ]
  }
}
```

- [ ] **Step 5: Add metrics endpoint**

Add to `internal/api/router.go`:

```go
import "github.com/prometheus/client_golang/prometheus/promhttp"

r.Get("/metrics", promhttp.Handler().ServeHTTP)
```

- [ ] **Step 6: Commit**

```bash
git add docker-compose.yml deploy/prometheus/ deploy/grafana/
git commit -m "feat(monitoring): add Prometheus and Grafana"
```

---

### Task 5: Alerting Rules

**Files:**
- Create: `deploy/prometheus/alerts.yml`

- [ ] **Step 1: Create alert rules**

```yaml
# deploy/prometheus/alerts.yml
groups:
  - name: aioj
    rules:
      - alert: HighErrorRate
        expr: rate(aioj_http_requests_total{status=~"5.."}[5m]) > 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          
      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(aioj_http_request_duration_seconds_bucket[5m])) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
          
      - alert: ServiceDown
        expr: up{job="aioj"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "AIOJ service is down"
```

- [ ] **Step 2: Commit**

```bash
git add deploy/prometheus/alerts.yml
git commit -m "feat(monitoring): add alerting rules"
```

---

## Verification Checklist

- [ ] /metrics endpoint returns Prometheus format
- [ ] /health endpoint returns service status
- [ ] Prometheus scrapes metrics
- [ ] Grafana shows dashboards
- [ ] Alerts fire on threshold breach

---

## Notes

1. **Metrics**: HTTP requests, submissions, contests, users
2. **Logging**: JSON format, structured fields
3. **Health**: Database and Redis connectivity
4. **Dashboards**: Request rate, latency, active contests
5. **Alerts**: Error rate, latency, service availability
