# Sub-Plan 19: Webhooks

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to register webhook endpoints for event notifications.

**Architecture:** Add webhooks table, webhook delivery service, retry logic.

**Tech Stack:** Go, PostgreSQL

---

## File Structure

### Backend Files to Create
- `internal/model/webhook.go` - Webhook models
- `internal/store/postgres/webhooks.go` - Webhook store
- `internal/webhook/delivery.go` - Webhook delivery service
- `internal/webhook/events.go` - Event types

### Backend Files to Modify
- `internal/store/interfaces.go` - Add WebhookStore interface

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000015_webhooks.up.sql`
- Create: `internal/store/migrations/000015_webhooks.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000015_webhooks.up.sql

CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url VARCHAR(512) NOT NULL,
    secret VARCHAR(128) NOT NULL,
    events TEXT[] NOT NULL DEFAULT '{}',
    is_active BOOLEAN NOT NULL DEFAULT true,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type VARCHAR(64) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    response_code INTEGER,
    response_body TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ
);

CREATE INDEX idx_webhooks_user ON webhooks(user_id);
CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status) WHERE status = 'pending';
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000015_webhooks.down.sql

DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhooks;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000015_webhooks.*
git commit -m "feat(webhooks): add webhooks database migration"
```

---

### Task 2: Webhook Models and Service

**Files:**
- Create: `internal/model/webhook.go`
- Create: `internal/webhook/events.go`
- Create: `internal/webhook/delivery.go`

- [ ] **Step 1: Create webhook models**

```go
// internal/model/webhook.go
package model

import "time"

type Webhook struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	URL         string    `json:"url"`
	Secret      string    `json:"-"`
	Events      []string  `json:"events"`
	IsActive    bool      `json:"is_active"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WebhookDelivery struct {
	ID           string     `json:"id"`
	WebhookID    string     `json:"webhook_id"`
	EventType    string     `json:"event_type"`
	Payload      any        `json:"payload"`
	Status       string     `json:"status"`
	ResponseCode *int       `json:"response_code,omitempty"`
	ResponseBody string     `json:"response_body,omitempty"`
	Attempts     int        `json:"attempts"`
	NextRetryAt  *time.Time `json:"next_retry_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
}

type CreateWebhookRequest struct {
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	Description string   `json:"description,omitempty"`
}

var WebhookEvents = []string{
	"submission.judged",
	"contest.started",
	"contest.ended",
	"rating.changed",
	"hack.received",
}
```

- [ ] **Step 2: Create event types**

```go
// internal/webhook/events.go
package webhook

type Event struct {
	Type      string    `json:"type"`
	Timestamp int64     `json:"timestamp"`
	Data      any       `json:"data"`
}

type SubmissionJudgedData struct {
	SubmissionID string `json:"submission_id"`
	ProblemID    string `json:"problem_id"`
	UserID       string `json:"user_id"`
	Status       string `json:"status"`
	Score        int    `json:"score"`
}

type RatingChangedData struct {
	UserID       string `json:"user_id"`
	OldRating    int    `json:"old_rating"`
	NewRating    int    `json:"new_rating"`
	ContestID    string `json:"contest_id"`
}
```

- [ ] **Step 3: Create delivery service**

```go
// internal/webhook/delivery.go
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type DeliveryService struct {
	webhookStore store.WebhookStore
	httpClient   *http.Client
}

func NewDeliveryService(ws store.WebhookStore) *DeliveryService {
	return &DeliveryService{
		webhookStore: ws,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *DeliveryService) Dispatch(ctx context.Context, eventType string, data any) error {
	// Get all active webhooks subscribed to this event
	webhooks, err := s.webhookStore.GetByEvent(ctx, eventType)
	if err != nil {
		return err
	}
	
	event := Event{
		Type:      eventType,
		Timestamp: time.Now().Unix(),
		Data:      data,
	}
	
	for _, wh := range webhooks {
		go s.deliver(wh, event)
	}
	
	return nil
}

func (s *DeliveryService) deliver(wh *model.Webhook, event Event) {
	payload, _ := json.Marshal(event)
	
	// Sign payload
	signature := signPayload(payload, wh.Secret)
	
	// Create request
	req, _ := http.NewRequest("POST", wh.URL, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AIOJ-Event", event.Type)
	req.Header.Set("X-AIOJ-Signature", signature)
	
	// Deliver with retries
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := s.httpClient.Do(req)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			// Success
			resp.Body.Close()
			return
		}
		
		if resp != nil {
			resp.Body.Close()
		}
		
		// Wait before retry
		time.Sleep(time.Duration(attempt+1) * 5 * time.Second)
	}
}

func signPayload(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/model/webhook.go internal/webhook/
git commit -m "feat(webhooks): add webhook models and delivery service"
```

---

### Task 3: Webhook Handler

**Files:**
- Create: `internal/api/handler/webhook.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create webhook handler**

```go
// internal/api/handler/webhook.go
package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type WebhookHandler struct {
	store *postgres.WebhookStore
}

func NewWebhookHandler(s *postgres.WebhookStore) *WebhookHandler {
	return &WebhookHandler{store: s}
}

func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	var req model.CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	// Generate secret
	secretBytes := make([]byte, 32)
	rand.Read(secretBytes)
	secret := hex.EncodeToString(secretBytes)
	
	wh := &model.Webhook{
		UserID:      claims.UserID,
		URL:         req.URL,
		Secret:      secret,
		Events:      req.Events,
		Description: req.Description,
		IsActive:    true,
	}
	
	if err := h.store.Create(r.Context(), wh); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"webhook": wh,
		"secret":  secret,
	})
}

func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	webhooks, err := h.store.GetByUser(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": webhooks})
}

func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), id); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
```

- [ ] **Step 2: Add routes**

```go
r.Route("/api/webhooks", func(r chi.Router) {
	r.Use(middleware.AuthMiddleware(jwtManager))
	r.Get("/", webhookH.List)
	r.Post("/", webhookH.Create)
	r.Delete("/{id}", webhookH.Delete)
})
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/webhook.go internal/api/router.go
git commit -m "feat(webhooks): add webhook API endpoints"
```

---

## Verification Checklist

- [ ] Webhooks can be created
- [ ] Secret generated correctly
- [ ] Events dispatched to webhooks
- [ ] Retry logic works
- [ ] Signature verification works

---

## Notes

1. **Events**: submission.judged, contest.started, contest.ended, rating.changed, hack.received
2. **Retry**: 3 attempts with exponential backoff
3. **Signature**: HMAC-SHA256 of payload
4. **Timeout**: 10 seconds per delivery
