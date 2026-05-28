# Sub-Plan 02: Contest Registration

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to register for contests before they start, with registration deadlines and participant lists.

**Architecture:** Add `contest_registrations` table, registration endpoints, frontend registration UI.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Create
- `internal/store/postgres/registrations.go` - Registration store
- `internal/api/handler/registration.go` - Registration handler

### Backend Files to Modify
- `internal/store/interfaces.go` - Add RegistrationStore interface
- `internal/api/router.go` - Add registration routes
- `internal/model/contest.go` - Add registration fields

### Frontend Files to Modify
- `web/src/pages/ContestDetail.tsx` - Add register button
- `web/src/pages/ContestList.tsx` - Show registration status
- `web/src/lib/api.ts` - Add registration API calls

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000004_contest_registration.up.sql`
- Create: `internal/store/migrations/000004_contest_registration.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000004_contest_registration.up.sql

CREATE TABLE contest_registrations (
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contest_id, user_id)
);

CREATE INDEX idx_contest_registrations_user ON contest_registrations(user_id);

-- Add registration fields to contests
ALTER TABLE contests ADD COLUMN IF NOT EXISTS registration_required BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS registration_deadline TIMESTAMPTZ;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS max_participants INTEGER;
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000004_contest_registration.down.sql

DROP TABLE IF EXISTS contest_registrations;
ALTER TABLE contests DROP COLUMN IF EXISTS registration_required;
ALTER TABLE contests DROP COLUMN IF EXISTS registration_deadline;
ALTER TABLE contests DROP COLUMN IF EXISTS max_participants;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: Migration applied successfully

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000004_contest_registration.*
git commit -m "feat(contest): add registration database migration"
```

---

### Task 2: Registration Store

**Files:**
- Create: `internal/store/postgres/registrations.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Add RegistrationStore interface**

Add to `internal/store/interfaces.go`:

```go
type RegistrationStore interface {
	Register(ctx context.Context, contestID, userID string) error
	Unregister(ctx context.Context, contestID, userID string) error
	IsRegistered(ctx context.Context, contestID, userID string) (bool, error)
	GetRegistrations(ctx context.Context, contestID string) ([]model.ContestRegistration, error)
	GetRegistrationCount(ctx context.Context, contestID string) (int, error)
}
```

- [ ] **Step 2: Add registration model**

Add to `internal/model/contest.go`:

```go
type ContestRegistration struct {
	ContestID    string    `json:"contest_id"`
	UserID       string    `json:"user_id"`
	Username     string    `json:"username,omitempty"`
	RegisteredAt time.Time `json:"registered_at"`
}
```

- [ ] **Step 3: Implement registration store**

```go
// internal/store/postgres/registrations.go
package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type RegistrationStore struct {
	db *sql.DB
}

func NewRegistrationStore(db *sql.DB) *RegistrationStore {
	return &RegistrationStore{db: db}
}

func (s *RegistrationStore) Register(ctx context.Context, contestID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO contest_registrations (contest_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		contestID, userID)
	return err
}

func (s *RegistrationStore) Unregister(ctx context.Context, contestID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM contest_registrations WHERE contest_id = $1 AND user_id = $2`,
		contestID, userID)
	return err
}

func (s *RegistrationStore) IsRegistered(ctx context.Context, contestID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM contest_registrations WHERE contest_id = $1 AND user_id = $2)`,
		contestID, userID).Scan(&exists)
	return exists, err
}

func (s *RegistrationStore) GetRegistrations(ctx context.Context, contestID string) ([]model.ContestRegistration, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT r.contest_id, r.user_id, u.username, r.registered_at
		 FROM contest_registrations r
		 JOIN users u ON r.user_id = u.id
		 WHERE r.contest_id = $1 ORDER BY r.registered_at`,
		contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var registrations []model.ContestRegistration
	for rows.Next() {
		var r model.ContestRegistration
		if err := rows.Scan(&r.ContestID, &r.UserID, &r.Username, &r.RegisteredAt); err != nil {
			return nil, err
		}
		registrations = append(registrations, r)
	}
	if registrations == nil {
		registrations = []model.ContestRegistration{}
	}
	return registrations, nil
}

func (s *RegistrationStore) GetRegistrationCount(ctx context.Context, contestID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM contest_registrations WHERE contest_id = $1`,
		contestID).Scan(&count)
	return count, err
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/store/interfaces.go internal/model/contest.go internal/store/postgres/registrations.go
git commit -m "feat(contest): add registration store"
```

---

### Task 3: Registration Handler

**Files:**
- Create: `internal/api/handler/registration.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create registration handler**

```go
// internal/api/handler/registration.go
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type RegistrationHandler struct {
	registrationStore *postgres.RegistrationStore
	contestStore      *postgres.ContestStore
}

func NewRegistrationHandler(rs *postgres.RegistrationStore, cs *postgres.ContestStore) *RegistrationHandler {
	return &RegistrationHandler{
		registrationStore: rs,
		contestStore:      cs,
	}
}

// Register registers the current user for a contest
func (h *RegistrationHandler) Register(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contestID := chi.URLParam(r, "id")
	contest, err := h.contestStore.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}

	// Check if registration is required
	if !contest.RegistrationRequired {
		http.Error(w, "registration not required for this contest", http.StatusBadRequest)
		return
	}

	// Check registration deadline
	if contest.RegistrationDeadline != nil && time.Now().After(*contest.RegistrationDeadline) {
		http.Error(w, "registration deadline passed", http.StatusBadRequest)
		return
	}

	// Check max participants
	if contest.MaxParticipants != nil {
		count, _ := h.registrationStore.GetRegistrationCount(r.Context(), contestID)
		if count >= *contest.MaxParticipants {
			http.Error(w, "contest is full", http.StatusBadRequest)
			return
		}
	}

	if err := h.registrationStore.Register(r.Context(), contestID, claims.UserID); err != nil {
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "registered"})
}

// Unregister removes the current user from a contest
func (h *RegistrationHandler) Unregister(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	contestID := chi.URLParam(r, "id")
	if err := h.registrationStore.Unregister(r.Context(), contestID, claims.UserID); err != nil {
		http.Error(w, "unregister failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "unregistered"})
}

// CheckRegistration checks if current user is registered
func (h *RegistrationHandler) CheckRegistration(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		respondJSON(w, http.StatusOK, map[string]bool{"registered": false})
		return
	}

	contestID := chi.URLParam(r, "id")
	registered, _ := h.registrationStore.IsRegistered(r.Context(), contestID, claims.UserID)
	respondJSON(w, http.StatusOK, map[string]bool{"registered": registered})
}

// ListRegistrations returns all registrations for a contest
func (h *RegistrationHandler) ListRegistrations(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "id")
	registrations, err := h.registrationStore.GetRegistrations(r.Context(), contestID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	count, _ := h.registrationStore.GetRegistrationCount(r.Context(), contestID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data":  registrations,
		"count": count,
	})
}
```

- [ ] **Step 2: Add routes to router**

Add to `internal/api/router.go`:

```go
r.Route("/api/contests/{id}/register", func(r chi.Router) {
	r.Get("/", registrationH.CheckRegistration)
	r.With(middleware.AuthMiddleware(jwtManager)).Post("/", registrationH.Register)
	r.With(middleware.AuthMiddleware(jwtManager)).Delete("/", registrationH.Unregister)
})

r.Get("/api/contests/{id}/registrations", registrationH.ListRegistrations)
```

Also add `registrationH *handler.RegistrationHandler` to `NewRouter` parameters.

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/registration.go internal/api/router.go
git commit -m "feat(contest): add registration API endpoints"
```

---

### Task 4: Update Contest Model

**Files:**
- Modify: `internal/model/contest.go`
- Modify: `internal/store/postgres/contests.go`

- [ ] **Step 1: Add registration fields to Contest model**

Add to `internal/model/contest.go`:

```go
type Contest struct {
	// ... existing fields ...
	RegistrationRequired bool       `json:"registration_required"`
	RegistrationDeadline *time.Time `json:"registration_deadline,omitempty"`
	MaxParticipants      *int       `json:"max_participants,omitempty"`
}
```

- [ ] **Step 2: Update contest store queries**

Update `internal/store/postgres/contests.go` to include new fields in queries.

- [ ] **Step 3: Commit**

```bash
git add internal/model/contest.go internal/store/postgres/contests.go
git commit -m "feat(contest): add registration fields to contest model"
```

---

### Task 5: Frontend Registration UI

**Files:**
- Modify: `web/src/pages/ContestDetail.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add registration API calls**

Add to `web/src/lib/api.ts`:

```typescript
contests: {
    // ... existing methods ...
    register: (id: string) => request(`/contests/${id}/register`, { method: 'POST' }),
    unregister: (id: string) => request(`/contests/${id}/register`, { method: 'DELETE' }),
    checkRegistration: (id: string) => request<{ registered: boolean }>(`/contests/${id}/register`),
    listRegistrations: (id: string) => request<{ data: any[]; count: number }>(`/contests/${id}/registrations`),
},
```

- [ ] **Step 2: Add registration UI to ContestDetail**

Add registration button and participant count to ContestDetail page:

```tsx
// Add state for registration
const [registered, setRegistered] = useState(false);
const [registrationCount, setRegistrationCount] = useState(0);

// Check registration status on load
useEffect(() => {
    if (id && getAccessToken()) {
        api.contests.checkRegistration(id).then(d => setRegistered(d.registered)).catch(() => {});
    }
    if (id) {
        api.contests.listRegistrations(id).then(d => setRegistrationCount(d.count)).catch(() => {});
    }
}, [id]);

// Add registration UI
{contest.registration_required && (
    <div className="border border-gray-200 rounded-lg p-4">
        <div className="flex items-center justify-between mb-2">
            <span className="text-sm text-gray-600">
                {registrationCount} / {contest.max_participants || '∞'} registered
            </span>
            {registered && <span className="text-green-600 text-sm">✓ Registered</span>}
        </div>
        {isUpcoming && (
            <button
                onClick={() => registered ? handleUnregister() : handleRegister()}
                className={`w-full py-2 rounded text-sm font-medium ${
                    registered 
                        ? 'bg-red-50 text-red-600 hover:bg-red-100' 
                        : 'bg-blue-600 text-white hover:bg-blue-700'
                }`}
            >
                {registered ? 'Unregister' : 'Register'}
            </button>
        )}
    </div>
)}
```

- [ ] **Step 3: Test the UI**

Run: `cd web && npm run dev`
Navigate to a contest with registration required and verify the registration UI works.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/ContestDetail.tsx web/src/lib/api.ts
git commit -m "feat(contest): add registration UI to contest detail page"
```

---

## Verification Checklist

After completing all tasks:

- [ ] Database migration runs successfully
- [ ] Users can register for contests
- [ ] Users can unregister from contests
- [ ] Registration status is checked correctly
- [ ] Registration deadline is enforced
- [ ] Max participants limit is enforced
- [ ] Registration count displays correctly
- [ ] UI updates after registration/unregistration

---

## Notes

1. **Registration deadline**: Must be before contest start time.
2. **Max participants**: Optional limit to prevent overload.
3. **Unregister**: Allowed until contest starts.
4. **Admin override**: Admins can bypass registration requirements.
