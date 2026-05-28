# Sub-Plan 07: Educational Rounds

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create educational contest format with extended hacking phases and editorial content.

**Architecture:** Extend contest model with educational round type, add extended hack phase, editorial integration.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Modify
- `internal/model/contest.go` - Add educational round type
- `internal/store/postgres/contests.go` - Add educational queries
- `internal/api/handler/contest.go` - Add educational round creation

### Frontend Files to Modify
- `web/src/pages/ContestDetail.tsx` - Show educational features
- `web/src/pages/ContestCreate.tsx` - Add educational round option

---

## Tasks

### Task 1: Add Educational Round Type

**Files:**
- Modify: `internal/model/contest.go`

- [ ] **Step 1: Add contest type constants**

Add to `internal/model/contest.go`:

```go
// Contest types
const (
	ContestTypeACM       = "acm"
	ContestTypeOI        = "oi"
	ContestTypeIOI       = "ioi"
	ContestTypePractice  = "practice"
	ContestTypeEducational = "educational"
)

// EducationalRoundConfig defines settings for educational rounds
type EducationalRoundConfig struct {
	HackPhaseHours    int  `json:"hack_phase_hours"`    // Default: 24
	ShowSolutions     bool `json:"show_solutions"`       // Show solutions after contest
	AllowUpsolving    bool `json:"allow_upsolving"`      // Allow solving after contest
	RatedForDivisions []int `json:"rated_for_divisions"` // Which divisions this is rated for
}

// DefaultEducationalConfig returns default educational round settings
func DefaultEducationalConfig() EducationalRoundConfig {
	return EducationalRoundConfig{
		HackPhaseHours:    24,
		ShowSolutions:     true,
		AllowUpsolving:    true,
		RatedForDivisions: []int{2, 3}, // Rated for Div 2 and Div 3
	}
}
```

- [ ] **Step 2: Add educational config to Contest model**

```go
type Contest struct {
	// ... existing fields ...
	Type               string                 `json:"type"`
	EducationalConfig  *EducationalRoundConfig `json:"educational_config,omitempty"`
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/model/contest.go
git commit -m "feat(edu): add educational round type and config"
```

---

### Task 2: Database Migration

**Files:**
- Create: `internal/store/migrations/000010_educational.up.sql`
- Create: `internal/store/migrations/000010_educational.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000010_educational.up.sql

-- Add educational config column
ALTER TABLE contests ADD COLUMN IF NOT EXISTS educational_config JSONB;

-- Add editorial table for educational rounds
CREATE TABLE IF NOT EXISTS editorials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    contest_id UUID REFERENCES contests(id),
    user_id UUID NOT NULL REFERENCES users(id),
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    solution_code TEXT,
    solution_language VARCHAR(64),
    is_official BOOLEAN NOT NULL DEFAULT false,
    upvotes INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_editorials_problem ON editorials(problem_id);
CREATE INDEX idx_editorials_contest ON editorials(contest_id) WHERE contest_id IS NOT NULL;
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000010_educational.down.sql

DROP TABLE IF EXISTS editorials;
ALTER TABLE contests DROP COLUMN IF EXISTS educational_config;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000010_educational.*
git commit -m "feat(edu): add educational round database migration"
```

---

### Task 3: Educational Round Handler

**Files:**
- Modify: `internal/api/handler/contest.go`

- [ ] **Step 1: Add educational round creation**

```go
func (h *ContestHandler) CreateEducational(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	
	var req struct {
		Title       string   `json:"title"`
		StartTime   time.Time `json:"start_time"`
		EndTime     time.Time `json:"end_time"`
		ProblemIDs  []string `json:"problem_ids"`
		Description string   `json:"description"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	// Create contest with educational type
	config := model.DefaultEducationalConfig()
	
	c := &model.Contest{
		ID:               uuid.New().String(),
		Title:            req.Title,
		Type:             model.ContestTypeEducational,
		StartTime:        req.StartTime,
		EndTime:          req.EndTime,
		EducationalConfig: &config,
		Description:      req.Description,
		Visible:          true,
		CreatedBy:        claims.UserID,
	}
	
	// Set hack phase end time (24 hours after contest ends)
	hackPhaseEnd := req.EndTime.Add(time.Duration(config.HackPhaseHours) * time.Hour)
	c.HackPhaseEnabled = true
	c.HackPhaseStart = &req.EndTime
	c.HackPhaseEnd = &hackPhaseEnd
	
	if err := h.store.Create(r.Context(), c); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	
	// Add problems
	for i, pid := range req.ProblemIDs {
		idx := string(rune('A' + i))
		h.store.AddProblem(r.Context(), c.ID, pid, idx, 100, i)
	}
	
	respondJSON(w, http.StatusCreated, c)
}
```

- [ ] **Step 2: Add route**

```go
r.Post("/api/contests/educational", contestH.CreateEducational)
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/contest.go internal/api/router.go
git commit -m "feat(edu): add educational round creation endpoint"
```

---

### Task 4: Frontend Educational Round UI

**Files:**
- Modify: `web/src/pages/ContestDetail.tsx`

- [ ] **Step 1: Add educational round indicators**

```tsx
// In ContestDetail, add educational round badge
{contest.type === 'educational' && (
  <span className="bg-green-100 text-green-800 px-2 py-1 rounded text-xs font-medium">
    Educational
  </span>
)}

// Add hack phase info for educational rounds
{contest.type === 'educational' && contest.hack_phase_enabled && (
  <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4 mt-4">
    <h3 className="font-semibold text-yellow-800 mb-2">Hack Phase</h3>
    <p className="text-sm text-yellow-700">
      After the contest ends, there will be a {contest.educational_config?.hack_phase_hours || 24}-hour 
      hacking phase where you can challenge other participants' solutions.
    </p>
  </div>
)}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/pages/ContestDetail.tsx
git commit -m "feat(edu): add educational round UI indicators"
```

---

## Verification Checklist

- [ ] Educational round type exists
- [ ] Hack phase is automatically set (24 hours)
- [ ] Educational badge displays
- [ ] Hack phase info shows
- [ ] Config can be customized

---

## Notes

1. **Hack Phase**: 24 hours after contest ends (configurable)
2. **Rated for**: Typically Div 2 and Div 3
3. **Editorials**: Solutions published after hack phase
4. **Upsolving**: Problems available for practice after contest
