# Sub-Plan 05: Virtual Contests

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to participate in past contests as if they were live, with realistic timing and ranking.

**Architecture:** Add `virtual_contests` table, virtual contest service, frontend virtual contest UI.

**Tech Stack:** Go, PostgreSQL, React, TypeScript, Redis (for timer state)

---

## File Structure

### Backend Files to Create
- `internal/model/virtual.go` - Virtual contest models
- `internal/store/postgres/virtual.go` - Virtual contest store
- `internal/virtual/service.go` - Virtual contest service
- `internal/api/handler/virtual.go` - Virtual contest handler

### Backend Files to Modify
- `internal/store/interfaces.go` - Add VirtualStore interface
- `internal/api/router.go` - Add virtual contest routes

### Frontend Files to Create
- `web/src/pages/VirtualContest.tsx` - Virtual contest page
- `web/src/components/VirtualTimer.tsx` - Timer component

### Frontend Files to Modify
- `web/src/App.tsx` - Add virtual contest route
- `web/src/lib/api.ts` - Add virtual contest API calls
- `web/src/pages/ContestDetail.tsx` - Add start virtual button

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000005_virtual_contests.up.sql`
- Create: `internal/store/migrations/000005_virtual_contests.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000005_virtual_contests.up.sql

CREATE TABLE virtual_contests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_contest_id UUID NOT NULL REFERENCES contests(id),
    user_id UUID NOT NULL REFERENCES users(id),
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    duration_minutes INTEGER NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'abandoned')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_virtual_contests_user ON virtual_contests(user_id);
CREATE INDEX idx_virtual_contests_original ON virtual_contests(original_contest_id);
CREATE INDEX idx_virtual_contests_status ON virtual_contests(status) WHERE status = 'active';
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000005_virtual_contests.down.sql

DROP TABLE IF EXISTS virtual_contests;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: Migration applied successfully

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000005_virtual_contests.*
git commit -m "feat(virtual): add virtual contests database migration"
```

---

### Task 2: Virtual Contest Models

**Files:**
- Create: `internal/model/virtual.go`

- [ ] **Step 1: Create virtual contest models**

```go
// internal/model/virtual.go
package model

import "time"

type VirtualContest struct {
	ID                 string     `json:"id"`
	OriginalContestID  string     `json:"original_contest_id"`
	UserID             string     `json:"user_id"`
	Username           string     `json:"username,omitempty"`
	StartedAt          time.Time  `json:"started_at"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	DurationMinutes    int        `json:"duration_minutes"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
}

type VirtualStanding struct {
	UserID       string                   `json:"user_id"`
	Username     string                   `json:"username"`
	Problems     map[string]ProblemResult `json:"problems"`
	TotalSolved  int                      `json:"total_solved"`
	TotalPenalty int                      `json:"total_penalty"`
	IsVirtual    bool                     `json:"is_virtual"`
}

type CreateVirtualRequest struct {
	ContestID       string `json:"contest_id"`
	DurationMinutes int    `json:"duration_minutes"`
}

type VirtualStatus struct {
	IsActive       bool       `json:"is_active"`
	VirtualID      string     `json:"virtual_id,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	EndsAt         *time.Time `json:"ends_at,omitempty"`
	RemainingMins  int        `json:"remaining_minutes"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/virtual.go
git commit -m "feat(virtual): add virtual contest models"
```

---

### Task 3: Virtual Contest Store

**Files:**
- Create: `internal/store/postgres/virtual.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Add VirtualStore interface**

Add to `internal/store/interfaces.go`:

```go
type VirtualStore interface {
	Create(ctx context.Context, v *model.VirtualContest) error
	GetByID(ctx context.Context, id string) (*model.VirtualContest, error)
	GetActiveByUser(ctx context.Context, userID string) (*model.VirtualContest, error)
	Complete(ctx context.Context, id string) error
	Abandon(ctx context.Context, id string) error
	GetByOriginalContest(ctx context.Context, contestID string) ([]model.VirtualContest, error)
}
```

- [ ] **Step 2: Implement virtual store**

```go
// internal/store/postgres/virtual.go
package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type VirtualStore struct {
	db *sql.DB
}

func NewVirtualStore(db *sql.DB) *VirtualStore {
	return &VirtualStore{db: db}
}

func (s *VirtualStore) Create(ctx context.Context, v *model.VirtualContest) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO virtual_contests (original_contest_id, user_id, started_at, duration_minutes, status)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		v.OriginalContestID, v.UserID, v.StartedAt, v.DurationMinutes, v.Status,
	).Scan(&v.ID, &v.CreatedAt)
}

func (s *VirtualStore) GetByID(ctx context.Context, id string) (*model.VirtualContest, error) {
	var v model.VirtualContest
	err := s.db.QueryRowContext(ctx,
		`SELECT id, original_contest_id, user_id, started_at, ended_at, duration_minutes, status, created_at
		 FROM virtual_contests WHERE id = $1`,
		id).Scan(&v.ID, &v.OriginalContestID, &v.UserID, &v.StartedAt, &v.EndedAt,
		&v.DurationMinutes, &v.Status, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *VirtualStore) GetActiveByUser(ctx context.Context, userID string) (*model.VirtualContest, error) {
	var v model.VirtualContest
	err := s.db.QueryRowContext(ctx,
		`SELECT id, original_contest_id, user_id, started_at, ended_at, duration_minutes, status, created_at
		 FROM virtual_contests WHERE user_id = $1 AND status = 'active'`,
		userID).Scan(&v.ID, &v.OriginalContestID, &v.UserID, &v.StartedAt, &v.EndedAt,
		&v.DurationMinutes, &v.Status, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *VirtualStore) Complete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE virtual_contests SET status = 'completed', ended_at = NOW() WHERE id = $1`,
		id)
	return err
}

func (s *VirtualStore) Abandon(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE virtual_contests SET status = 'abandoned', ended_at = NOW() WHERE id = $1`,
		id)
	return err
}

func (s *VirtualStore) GetByOriginalContest(ctx context.Context, contestID string) ([]model.VirtualContest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT v.id, v.original_contest_id, v.user_id, u.username, v.started_at, v.ended_at, 
		        v.duration_minutes, v.status, v.created_at
		 FROM virtual_contests v
		 JOIN users u ON v.user_id = u.id
		 WHERE v.original_contest_id = $1
		 ORDER BY v.created_at DESC`,
		contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var virtuals []model.VirtualContest
	for rows.Next() {
		var v model.VirtualContest
		if err := rows.Scan(&v.ID, &v.OriginalContestID, &v.UserID, &v.Username, &v.StartedAt,
			&v.EndedAt, &v.DurationMinutes, &v.Status, &v.CreatedAt); err != nil {
			return nil, err
		}
		virtuals = append(virtuals, v)
	}
	if virtuals == nil {
		virtuals = []model.VirtualContest{}
	}
	return virtuals, nil
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/virtual.go
git commit -m "feat(virtual): add virtual contest store"
```

---

### Task 4: Virtual Contest Service

**Files:**
- Create: `internal/virtual/service.go`
- Create: `internal/virtual/service_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/virtual/service_test.go
package virtual

import (
	"testing"
	"time"
)

func TestCalculateRemainingTime(t *testing.T) {
	tests := []struct {
		name     string
		started  time.Time
		duration int
		now      time.Time
		expected int
	}{
		{
			name:     "just started",
			started:  time.Now(),
			duration: 120,
			now:      time.Now(),
			expected: 120,
		},
		{
			name:     "half done",
			started:  time.Now().Add(-60 * time.Minute),
			duration: 120,
			now:      time.Now(),
			expected: 60,
		},
		{
			name:     "expired",
			started:  time.Now().Add(-130 * time.Minute),
			duration: 120,
			now:      time.Now(),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRemainingMinutes(tt.started, tt.duration, tt.now)
			if result != tt.expected {
				t.Errorf("CalculateRemainingMinutes() = %d, want %d", result, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Implement virtual service**

```go
// internal/virtual/service.go
package virtual

import (
	"context"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type Service struct {
	virtualStore store.VirtualStore
	contestStore store.ContestStore
}

func NewService(vs store.VirtualStore, cs store.ContestStore) *Service {
	return &Service{
		virtualStore: vs,
		contestStore: cs,
	}
}

// StartVirtualContest creates a new virtual contest session
func (s *Service) StartVirtualContest(ctx context.Context, userID, contestID string, durationMinutes int) (*model.VirtualContest, error) {
	// Check if user already has an active virtual contest
	existing, _ := s.virtualStore.GetActiveByUser(ctx, userID)
	if existing != nil {
		return nil, ErrActiveVirtualExists
	}

	v := &model.VirtualContest{
		OriginalContestID: contestID,
		UserID:            userID,
		StartedAt:         time.Now(),
		DurationMinutes:   durationMinutes,
		Status:            "active",
	}

	if err := s.virtualStore.Create(ctx, v); err != nil {
		return nil, err
	}

	return v, nil
}

// GetStatus returns the current status of a virtual contest
func (s *Service) GetStatus(v *model.VirtualContest, now time.Time) model.VirtualStatus {
	endsAt := v.StartedAt.Add(time.Duration(v.DurationMinutes) * time.Minute)
	remaining := CalculateRemainingMinutes(v.StartedAt, v.DurationMinutes, now)

	return model.VirtualStatus{
		IsActive:      v.Status == "active" && remaining > 0,
		VirtualID:     v.ID,
		StartedAt:     &v.StartedAt,
		EndsAt:        &endsAt,
		RemainingMins: remaining,
	}
}

// CompleteContest marks a virtual contest as completed
func (s *Service) CompleteContest(ctx context.Context, virtualID string) error {
	return s.virtualStore.Complete(ctx, virtualID)
}

// CalculateRemainingMinutes returns how many minutes are left
func CalculateRemainingMinutes(startedAt time.Time, durationMinutes int, now time.Time) int {
	endsAt := startedAt.Add(time.Duration(durationMinutes) * time.Minute)
	remaining := endsAt.Sub(now).Minutes()
	if remaining < 0 {
		return 0
	}
	return int(remaining)
}

var ErrActiveVirtualExists = fmt.Errorf("you already have an active virtual contest")
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/virtual/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/virtual/service.go internal/virtual/service_test.go
git commit -m "feat(virtual): add virtual contest service"
```

---

### Task 5: Virtual Contest Handler

**Files:**
- Create: `internal/api/handler/virtual.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create virtual handler**

```go
// internal/api/handler/virtual.go
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/virtual"
)

type VirtualHandler struct {
	service *virtual.Service
}

func NewVirtualHandler(s *virtual.Service) *VirtualHandler {
	return &VirtualHandler{service: s}
}

// Start creates a new virtual contest
func (h *VirtualHandler) Start(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ContestID       string `json:"contest_id"`
		DurationMinutes int    `json:"duration_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.DurationMinutes <= 0 {
		req.DurationMinutes = 120 // Default 2 hours
	}

	v, err := h.service.StartVirtualContest(r.Context(), claims.UserID, req.ContestID, req.DurationMinutes)
	if err != nil {
		if err == virtual.ErrActiveVirtualExists {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "failed to start virtual contest", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusCreated, v)
}

// Status returns current virtual contest status
func (h *VirtualHandler) Status(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		respondJSON(w, http.StatusOK, map[string]bool{"is_active": false})
		return
	}

	v, err := h.service.GetActiveByUser(r.Context(), claims.UserID)
	if err != nil || v == nil {
		respondJSON(w, http.StatusOK, map[string]bool{"is_active": false})
		return
	}

	status := h.service.GetStatus(v, time.Now())
	respondJSON(w, http.StatusOK, status)
}

// Complete marks virtual contest as done
func (h *VirtualHandler) Complete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	virtualID := chi.URLParam(r, "id")
	if err := h.service.CompleteContest(r.Context(), virtualID); err != nil {
		http.Error(w, "failed to complete", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}
```

- [ ] **Step 2: Add routes**

Add to `internal/api/router.go`:

```go
r.Route("/api/virtual", func(r chi.Router) {
	r.Use(middleware.AuthMiddleware(jwtManager))
	r.Post("/start", virtualH.Start)
	r.Get("/status", virtualH.Status)
	r.Post("/{id}/complete", virtualH.Complete)
})
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/virtual.go internal/api/router.go
git commit -m "feat(virtual): add virtual contest API endpoints"
```

---

### Task 6: Frontend Virtual Contest Page

**Files:**
- Create: `web/src/pages/VirtualContest.tsx`
- Create: `web/src/components/VirtualTimer.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add virtual API calls**

Add to `web/src/lib/api.ts`:

```typescript
virtual: {
    start: (contestId: string, durationMinutes?: number) =>
        request<any>('/virtual/start', {
            method: 'POST',
            body: JSON.stringify({ contest_id: contestId, duration_minutes: durationMinutes }),
        }),
    status: () => request<any>('/virtual/status'),
    complete: (id: string) => request(`/virtual/${id}/complete`, { method: 'POST' }),
},
```

- [ ] **Step 2: Create VirtualTimer component**

```tsx
// web/src/components/VirtualTimer.tsx
import { useEffect, useState } from 'react';

interface VirtualTimerProps {
  endsAt: string;
  onComplete: () => void;
}

export default function VirtualTimer({ endsAt, onComplete }: VirtualTimerProps) {
  const [remaining, setRemaining] = useState(0);

  useEffect(() => {
    const update = () => {
      const end = new Date(endsAt).getTime();
      const now = Date.now();
      const diff = Math.max(0, Math.floor((end - now) / 1000));
      setRemaining(diff);
      if (diff === 0) onComplete();
    };

    update();
    const interval = setInterval(update, 1000);
    return () => clearInterval(interval);
  }, [endsAt, onComplete]);

  const hours = Math.floor(remaining / 3600);
  const minutes = Math.floor((remaining % 3600) / 60);
  const seconds = remaining % 60;

  const isUrgent = remaining < 300; // Last 5 minutes

  return (
    <div className={`font-mono text-2xl font-bold ${isUrgent ? 'text-red-600 animate-pulse' : 'text-gray-800'}`}>
      {String(hours).padStart(2, '0')}:{String(minutes).padStart(2, '0')}:{String(seconds).padStart(2, '0')}
    </div>
  );
}
```

- [ ] **Step 3: Create VirtualContest page**

```tsx
// web/src/pages/VirtualContest.tsx
import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import VirtualTimer from '../components/VirtualTimer';

export default function VirtualContest() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [virtual, setVirtual] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.virtual.status().then(setVirtual).catch(() => {}).finally(() => setLoading(false));
  }, []);

  const handleComplete = async () => {
    if (virtual?.virtual_id) {
      await api.virtual.complete(virtual.virtual_id);
      navigate(`/contests/${id}/scoreboard`);
    }
  };

  if (loading) return <div>Loading...</div>;
  if (!virtual?.is_active) return <div>No active virtual contest</div>;

  return (
    <div className="space-y-6">
      <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
        <h2 className="text-lg font-semibold text-yellow-800 mb-2">Virtual Contest in Progress</h2>
        <div className="flex items-center justify-between">
          <VirtualTimer endsAt={virtual.ends_at} onComplete={handleComplete} />
          <button
            onClick={handleComplete}
            className="bg-red-600 text-white px-4 py-2 rounded hover:bg-red-700"
          >
            End Contest
          </button>
        </div>
      </div>
      {/* Contest problems would go here */}
    </div>
  );
}
```

- [ ] **Step 4: Add route to App.tsx**

Add to routes:
```tsx
<Route path="/virtual/:id" element={<VirtualContest />} />
```

- [ ] **Step 5: Add Start Virtual button to ContestDetail**

Add to ContestDetail page for ended contests:
```tsx
{isEnded && (
    <button
        onClick={() => startVirtual()}
        className="bg-purple-600 text-white px-4 py-2 rounded hover:bg-purple-700"
    >
        Start Virtual Contest
    </button>
)}
```

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/VirtualContest.tsx web/src/components/VirtualTimer.tsx web/src/App.tsx web/src/lib/api.ts web/src/pages/ContestDetail.tsx
git commit -m "feat(virtual): add virtual contest frontend"
```

---

## Verification Checklist

After completing all tasks:

- [ ] Database migration runs successfully
- [ ] Users can start virtual contests
- [ ] Timer counts down correctly
- [ ] Virtual contest completes automatically when time expires
- [ ] Users can manually end virtual contest
- [ ] Submissions are isolated to virtual contest
- [ ] Virtual standings are calculated separately
- [ ] Only one active virtual per user

---

## Notes

1. **Duration**: Default 2 hours, customizable.
2. **Isolation**: Virtual submissions don't affect original contest.
3. **Ranking**: Virtual participants ranked together.
4. **Ghost participants**: Future enhancement to show original participants.
