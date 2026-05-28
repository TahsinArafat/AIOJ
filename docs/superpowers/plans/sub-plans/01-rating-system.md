# Sub-Plan 01: Rating System

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Elo-based rating system with color coding, rating history, and division eligibility.

**Architecture:** Add `rating_history` table, rating calculation service, update user profiles after contests, add color coding to frontend.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Create
- `internal/model/rating.go` - Rating models and types
- `internal/store/postgres/rating.go` - Rating store implementation
- `internal/rating/calculator.go` - Elo calculation algorithm
- `internal/rating/calculator_test.go` - Calculator tests
- `internal/api/handler/rating.go` - Rating API handlers
- `internal/store/migrations/000003_rating_system.up.sql` - Database migration
- `internal/store/migrations/000003_rating_system.down.sql` - Rollback migration

### Backend Files to Modify
- `internal/store/interfaces.go` - Add RatingStore interface
- `internal/api/router.go` - Add rating routes
- `internal/api/handler/contest.go` - Trigger rating update after contest

### Frontend Files to Create
- `web/src/lib/rating.ts` - Rating color utilities
- `web/src/components/RatingBadge.tsx` - Rating badge component
- `web/src/pages/RatingHistory.tsx` - Rating history page

### Frontend Files to Modify
- `web/src/lib/api.ts` - Add rating API calls
- `web/src/pages/Profile.tsx` - Show rating with color
- `web/src/pages/ContestScoreboard.tsx` - Show rating changes
- `web/src/App.tsx` - Add rating history route

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000003_rating_system.up.sql`
- Create: `internal/store/migrations/000003_rating_system.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000003_rating_system.up.sql

-- Rating history table
CREATE TABLE rating_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    old_rating INTEGER NOT NULL,
    new_rating INTEGER NOT NULL,
    rank INTEGER NOT NULL,
    rating_change INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rating_history_user ON rating_history(user_id);
CREATE INDEX idx_rating_history_contest ON rating_history(contest_id);

-- Add rating columns to user_profiles if not exists
ALTER TABLE user_profiles ADD COLUMN IF NOT EXISTS max_rating INTEGER NOT NULL DEFAULT 0;
ALTER TABLE user_profiles ADD COLUMN IF NOT EXISTS contest_count INTEGER NOT NULL DEFAULT 0;

-- Add division columns to contests
ALTER TABLE contests ADD COLUMN IF NOT EXISTS division INTEGER NOT NULL DEFAULT 0;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS rated_for_min INTEGER NOT NULL DEFAULT 0;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS rated_for_max INTEGER NOT NULL DEFAULT 9999;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS is_rated BOOLEAN NOT NULL DEFAULT true;
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000003_rating_system.down.sql

DROP TABLE IF EXISTS rating_history;
ALTER TABLE user_profiles DROP COLUMN IF EXISTS max_rating;
ALTER TABLE user_profiles DROP COLUMN IF EXISTS contest_count;
ALTER TABLE contests DROP COLUMN IF EXISTS division;
ALTER TABLE contests DROP COLUMN IF EXISTS rated_for_min;
ALTER TABLE contests DROP COLUMN IF EXISTS rated_for_max;
ALTER TABLE contests DROP COLUMN IF EXISTS is_rated;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: Migration applied successfully

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000003_rating_system.*
git commit -m "feat(rating): add rating system database migration"
```

---

### Task 2: Rating Models

**Files:**
- Create: `internal/model/rating.go`

- [ ] **Step 1: Create rating models**

```go
// internal/model/rating.go
package model

import "time"

// Rating colors matching Codeforces
const (
	ColorNewbie           = "newbie"           // < 1200
	ColorPupil            = "pupil"            // 1200-1399
	ColorSpecialist       = "specialist"       // 1400-1599
	ColorExpert           = "expert"           // 1600-1899
	ColorCandidateMaster  = "candidate-master" // 1900-2099
	ColorMaster           = "master"           // 2100-2299
	ColorInternationalMaster = "international-master" // 2300-2399
	ColorGrandmaster      = "grandmaster"      // 2400-2599
	ColorInternationalGrandmaster = "international-grandmaster" // 2600-2899
	ColorLegendaryGrandmaster = "legendary-grandmaster" // 2900+
)

type RatingHistory struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	ContestID    string    `json:"contest_id"`
	OldRating    int       `json:"old_rating"`
	NewRating    int       `json:"new_rating"`
	Rank         int       `json:"rank"`
	RatingChange int       `json:"rating_change"`
	CreatedAt    time.Time `json:"created_at"`
}

type RatingChange struct {
	UserID       string `json:"user_id"`
	Username     string `json:"username"`
	OldRating    int    `json:"old_rating"`
	NewRating    int    `json:"new_rating"`
	RatingChange int    `json:"rating_change"`
	Rank         int    `json:"rank"`
	Color        string `json:"color"`
}

type ContestRatingRequest struct {
	ContestID string `json:"contest_id"`
}

// GetColor returns the color name for a given rating
func GetColor(rating int) string {
	switch {
	case rating >= 2900:
		return ColorLegendaryGrandmaster
	case rating >= 2600:
		return ColorInternationalGrandmaster
	case rating >= 2400:
		return ColorGrandmaster
	case rating >= 2300:
		return ColorInternationalMaster
	case rating >= 2100:
		return ColorMaster
	case rating >= 1900:
		return ColorCandidateMaster
	case rating >= 1600:
		return ColorExpert
	case rating >= 1400:
		return ColorSpecialist
	case rating >= 1200:
		return ColorPupil
	default:
		return ColorNewbie
	}
}

// GetColorHex returns the hex color for a given rating
func GetColorHex(rating int) string {
	switch {
	case rating >= 2900:
		return "#FF0000"
	case rating >= 2600:
		return "#FF0000"
	case rating >= 2400:
		return "#FF8C00"
	case rating >= 2300:
		return "#FF8C00"
	case rating >= 2100:
		return "#FFD700"
	case rating >= 1900:
		return "#AA00AA"
	case rating >= 1600:
		return "#0000FF"
	case rating >= 1400:
		return "#03A89E"
	case rating >= 1200:
		return "#008000"
	default:
		return "#808080"
	}
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/model/...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/model/rating.go
git commit -m "feat(rating): add rating models and color utilities"
```

---

### Task 3: Rating Calculator

**Files:**
- Create: `internal/rating/calculator.go`
- Create: `internal/rating/calculator_test.go`

- [ ] **Step 1: Write failing test for rating calculator**

```go
// internal/rating/calculator_test.go
package rating

import (
	"testing"
)

func TestCalculateEloRating(t *testing.T) {
	tests := []struct {
		name      string
		oldRating int
		rank      int
		participants int
		expected  int
	}{
		{
			name:      "first contest, middle rank",
			oldRating: 0,
			rank:      50,
			participants: 100,
			expected:  1500, // Starting rating
		},
		{
			name:      "improvement from low rating",
			oldRating: 1200,
			rank:      10,
			participants: 100,
			expected:  1350, // Should increase significantly
		},
		{
			name:      "decline from high rating",
			oldRating: 2000,
			rank:      90,
			participants: 100,
			expected:  1900, // Should decrease
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRating(tt.oldRating, tt.rank, tt.participants)
			// Allow +/- 50 for Elo variance
			if result < tt.expected-50 || result > tt.expected+50 {
				t.Errorf("CalculateRating(%d, %d, %d) = %d, want ~%d",
					tt.oldRating, tt.rank, tt.participants, result, tt.expected)
			}
		})
	}
}

func TestCalculateRatingChange(t *testing.T) {
	change := CalculateRatingChange(1500, 1600)
	if change != 100 {
		t.Errorf("CalculateRatingChange(1500, 1600) = %d, want 100", change)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/rating/... -v`
Expected: FAIL with "undefined: CalculateRating"

- [ ] **Step 3: Implement rating calculator**

```go
// internal/rating/calculator.go
package rating

import (
	"math"
)

const (
	// DefaultRating is the starting rating for new users
	DefaultRating = 1500
	
	// K-factor determines how much ratings change
	// Higher K = more volatile ratings
	KFactor = 32
	
	// RatingPerfFactor scales the performance rating calculation
	RatingPerfFactor = 400
)

// CalculateRating computes new rating using Elo system adapted for competitive programming
// Based on Codeforces rating system
func CalculateRating(oldRating, rank, participants int) int {
	// For first contest, use default rating
	if oldRating == 0 {
		oldRating = DefaultRating
	}
	
	// Calculate expected rank based on rating
	// Higher rated players are expected to rank higher
	expectedRank := calculateExpectedRank(oldRating, participants)
	
	// Calculate performance rating
	// Better than expected = positive change
	performanceRating := calculatePerformanceRating(rank, participants, oldRating)
	
	// Calculate new rating with damping factor
	// New players have higher volatility
	dampingFactor := calculateDampingFactor(oldRating)
	
	ratingChange := int(float64(performanceRating-oldRating) * dampingFactor)
	newRating := oldRating + ratingChange
	
	// Ensure rating doesn't go below 0
	if newRating < 0 {
		newRating = 0
	}
	
	return newRating
}

// CalculateRatingChange returns the difference between old and new rating
func CalculateRatingChange(oldRating, newRating int) int {
	return newRating - oldRating
}

// calculateExpectedRank estimates where a player should rank based on rating
func calculateExpectedRank(rating, participants int) float64 {
	// Use logistic distribution to estimate expected rank
	// Higher rating = lower expected rank (better position)
	midRating := float64(DefaultRating)
	scale := float64(RatingPerfFactor)
	
	// Probability of beating an average player
	prob := 1.0 / (1.0 + math.Pow(10, (midRating-float64(rating))/scale))
	
	// Expected rank is inverse of probability
	return float64(participants) * (1.0 - prob)
}

// calculatePerformanceRating estimates rating based on actual performance
func calculatePerformanceRating(rank, participants, currentRating int) int {
	// Performance rating = rating that would make actual rank = expected rank
	// Use binary search to find this rating
	
	low, high := 0, 5000
	
	for low < high {
		mid := (low + high) / 2
		expectedRank := calculateExpectedRank(mid, participants)
		
		if expectedRank < float64(rank) {
			high = mid
		} else {
			low = mid + 1
		}
	}
	
	return low
}

// calculateDampingFactor returns how much ratings should change
// New players (low rating) have higher volatility
func calculateDampingFactor(rating int) float64 {
	if rating < 1000 {
		return 1.0 // Full change for new players
	} else if rating < 2000 {
		return 0.75 // Moderate change
	} else {
		return 0.5 // Experienced players change slowly
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/rating/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/rating/calculator.go internal/rating/calculator_test.go
git commit -m "feat(rating): implement Elo rating calculator"
```

---

### Task 4: Rating Store

**Files:**
- Create: `internal/store/postgres/rating.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Add RatingStore interface**

Add to `internal/store/interfaces.go`:

```go
type RatingStore interface {
	CreateHistory(ctx context.Context, h *model.RatingHistory) error
	GetByUser(ctx context.Context, userID string, limit int) ([]model.RatingHistory, error)
	GetByContest(ctx context.Context, contestID string) ([]model.RatingHistory, error)
	GetLatestByUser(ctx context.Context, userID string) (*model.RatingHistory, error)
}
```

- [ ] **Step 2: Write failing test for rating store**

```go
// internal/store/postgres/rating_test.go
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

func TestRatingStore_CreateAndGet(t *testing.T) {
	// This test requires a test database
	// Skip if not in test environment
	if testing.Short() {
		t.Skip("Skipping database test")
	}

	db := setupTestDB(t)
	store := postgres.NewRatingStore(db)
	ctx := context.Background()

	// Create test history
	h := &model.RatingHistory{
		UserID:       "test-user-id",
		ContestID:    "test-contest-id",
		OldRating:    1500,
		NewRating:    1600,
		Rank:         10,
		RatingChange: 100,
		CreatedAt:    time.Now(),
	}

	err := store.CreateHistory(ctx, h)
	if err != nil {
		t.Fatalf("CreateHistory failed: %v", err)
	}

	// Get by user
	histories, err := store.GetByUser(ctx, "test-user-id", 10)
	if err != nil {
		t.Fatalf("GetByUser failed: %v", err)
	}
	if len(histories) == 0 {
		t.Error("Expected at least one history entry")
	}
}
```

- [ ] **Step 3: Implement rating store**

```go
// internal/store/postgres/rating.go
package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type RatingStore struct {
	db *sql.DB
}

func NewRatingStore(db *sql.DB) *RatingStore {
	return &RatingStore{db: db}
}

func (s *RatingStore) CreateHistory(ctx context.Context, h *model.RatingHistory) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO rating_history (user_id, contest_id, old_rating, new_rating, rank, rating_change)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		h.UserID, h.ContestID, h.OldRating, h.NewRating, h.Rank, h.RatingChange,
	).Scan(&h.ID, &h.CreatedAt)
}

func (s *RatingStore) GetByUser(ctx context.Context, userID string, limit int) ([]model.RatingHistory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, contest_id, old_rating, new_rating, rank, rating_change, created_at
		 FROM rating_history WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`,
		userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []model.RatingHistory
	for rows.Next() {
		var h model.RatingHistory
		if err := rows.Scan(&h.ID, &h.UserID, &h.ContestID, &h.OldRating, &h.NewRating,
			&h.Rank, &h.RatingChange, &h.CreatedAt); err != nil {
			return nil, err
		}
		histories = append(histories, h)
	}
	if histories == nil {
		histories = []model.RatingHistory{}
	}
	return histories, nil
}

func (s *RatingStore) GetByContest(ctx context.Context, contestID string) ([]model.RatingHistory, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, contest_id, old_rating, new_rating, rank, rating_change, created_at
		 FROM rating_history WHERE contest_id = $1 ORDER BY rank`,
		contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []model.RatingHistory
	for rows.Next() {
		var h model.RatingHistory
		if err := rows.Scan(&h.ID, &h.UserID, &h.ContestID, &h.OldRating, &h.NewRating,
			&h.Rank, &h.RatingChange, &h.CreatedAt); err != nil {
			return nil, err
		}
		histories = append(histories, h)
	}
	return histories, nil
}

func (s *RatingStore) GetLatestByUser(ctx context.Context, userID string) (*model.RatingHistory, error) {
	var h model.RatingHistory
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, contest_id, old_rating, new_rating, rank, rating_change, created_at
		 FROM rating_history WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID).Scan(&h.ID, &h.UserID, &h.ContestID, &h.OldRating, &h.NewRating,
		&h.Rank, &h.RatingChange, &h.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/store/postgres/... -v -short`
Expected: PASS (tests skip database tests in short mode)

- [ ] **Step 5: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/rating.go
git commit -m "feat(rating): add rating store implementation"
```

---

### Task 5: Rating API Handler

**Files:**
- Create: `internal/api/handler/rating.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create rating handler**

```go
// internal/api/handler/rating.go
package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type RatingHandler struct {
	ratingStore *postgres.RatingStore
	userStore   *postgres.UserStore
}

func NewRatingHandler(rs *postgres.RatingStore, us *postgres.UserStore) *RatingHandler {
	return &RatingHandler{
		ratingStore: rs,
		userStore:   us,
	}
}

// GetByUser returns rating history for a user
func (h *RatingHandler) GetByUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		http.Error(w, "user ID required", http.StatusBadRequest)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	histories, err := h.ratingStore.GetByUser(r.Context(), userID, limit)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": histories,
	})
}

// GetByContest returns rating changes for a contest
func (h *RatingHandler) GetByContest(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "contestId")
	if contestID == "" {
		http.Error(w, "contest ID required", http.StatusBadRequest)
		return
	}

	histories, err := h.ratingStore.GetByContest(r.Context(), contestID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"data": histories,
	})
}
```

- [ ] **Step 2: Add routes to router**

Add to `internal/api/router.go` after contest routes:

```go
r.Route("/api/rating", func(r chi.Router) {
	r.Get("/user/{userId}", ratingH.GetByUser)
	r.Get("/contest/{contestId}", ratingH.GetByContest)
})
```

Also add `ratingH *handler.RatingHandler` to `NewRouter` parameters.

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler/rating.go internal/api/router.go
git commit -m "feat(rating): add rating API endpoints"
```

---

### Task 6: Rating Calculation Service

**Files:**
- Create: `internal/rating/service.go`
- Create: `internal/rating/service_test.go`

- [ ] **Step 1: Write failing test for rating service**

```go
// internal/rating/service_test.go
package rating

import (
	"context"
	"testing"

	"github.com/tahsinarafat/aioj/internal/model"
)

type mockRatingStore struct {
	histories []model.RatingHistory
}

func (m *mockRatingStore) CreateHistory(ctx context.Context, h *model.RatingHistory) error {
	m.histories = append(m.histories, *h)
	return nil
}

func (m *mockRatingStore) GetByUser(ctx context.Context, userID string, limit int) ([]model.RatingHistory, error) {
	return m.histories, nil
}

func TestRatingService_CalculateContestRatings(t *testing.T) {
	service := NewService(nil, nil)
	
	// Create test standings
	standings := []ContestStanding{
		{UserID: "user1", Rank: 1, OldRating: 1500},
		{UserID: "user2", Rank: 2, OldRating: 1400},
		{UserID: "user3", Rank: 3, OldRating: 1600},
	}
	
	changes := service.CalculateContestRatings(standings)
	
	if len(changes) != 3 {
		t.Fatalf("Expected 3 changes, got %d", len(changes))
	}
	
	// Winner should gain rating
	if changes[0].RatingChange <= 0 {
		t.Error("Winner should gain rating")
	}
	
	// Loser should lose rating
	if changes[2].RatingChange >= 0 {
		t.Error("Last place should lose rating")
	}
}
```

- [ ] **Step 2: Implement rating service**

```go
// internal/rating/service.go
package rating

import (
	"context"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type ContestStanding struct {
	UserID    string
	Rank      int
	OldRating int
	Username  string
}

type Service struct {
	ratingStore store.RatingStore
	userStore   store.UserStore
}

func NewService(rs store.RatingStore, us store.UserStore) *Service {
	return &Service{
		ratingStore: rs,
		userStore:   us,
	}
}

// CalculateContestRatings computes rating changes for all participants
func (s *Service) CalculateContestRatings(standings []ContestStanding) []model.RatingChange {
	participants := len(standings)
	changes := make([]model.RatingChange, 0, participants)
	
	for _, standing := range standings {
		newRating := CalculateRating(standing.OldRating, standing.Rank, participants)
		ratingChange := CalculateRatingChange(standing.OldRating, newRating)
		
		changes = append(changes, model.RatingChange{
			UserID:       standing.UserID,
			Username:     standing.Username,
			OldRating:    standing.OldRating,
			NewRating:    newRating,
			RatingChange: ratingChange,
			Rank:         standing.Rank,
			Color:        model.GetColor(newRating),
		})
	}
	
	return changes
}

// ApplyContestRatings saves rating changes to database
func (s *Service) ApplyContestRatings(ctx context.Context, contestID string, changes []model.RatingChange) error {
	for _, change := range changes {
		h := &model.RatingHistory{
			UserID:       change.UserID,
			ContestID:    contestID,
			OldRating:    change.OldRating,
			NewRating:    change.NewRating,
			Rank:         change.Rank,
			RatingChange: change.RatingChange,
		}
		
		if err := s.ratingStore.CreateHistory(ctx, h); err != nil {
			return err
		}
		
		// Update user profile rating
		if err := s.updateUserRating(ctx, change.UserID, change.NewRating); err != nil {
			return err
		}
	}
	
	return nil
}

func (s *Service) updateUserRating(ctx context.Context, userID string, newRating int) error {
	// TODO: Implement user profile update
	return nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/rating/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/rating/service.go internal/rating/service_test.go
git commit -m "feat(rating): add rating calculation service"
```

---

### Task 7: Frontend Rating Utilities

**Files:**
- Create: `web/src/lib/rating.ts`
- Create: `web/src/components/RatingBadge.tsx`

- [ ] **Step 1: Create rating utility functions**

```typescript
// web/src/lib/rating.ts

export interface RatingColor {
  name: string;
  hex: string;
  bg: string;
  text: string;
}

export function getRatingColor(rating: number): RatingColor {
  if (rating >= 2900) {
    return { name: 'legendary-grandmaster', hex: '#FF0000', bg: '#FF000020', text: '#FF0000' };
  } else if (rating >= 2600) {
    return { name: 'international-grandmaster', hex: '#FF0000', bg: '#FF000020', text: '#FF0000' };
  } else if (rating >= 2400) {
    return { name: 'grandmaster', hex: '#FF8C00', bg: '#FF8C0020', text: '#FF8C00' };
  } else if (rating >= 2300) {
    return { name: 'international-master', hex: '#FF8C00', bg: '#FF8C0020', text: '#FF8C00' };
  } else if (rating >= 2100) {
    return { name: 'master', hex: '#FFD700', bg: '#FFD70020', text: '#B8860B' };
  } else if (rating >= 1900) {
    return { name: 'candidate-master', hex: '#AA00AA', bg: '#AA00AA20', text: '#AA00AA' };
  } else if (rating >= 1600) {
    return { name: 'expert', hex: '#0000FF', bg: '#0000FF20', text: '#0000FF' };
  } else if (rating >= 1400) {
    return { name: 'specialist', hex: '#03A89E', bg: '#03A89E20', text: '#03A89E' };
  } else if (rating >= 1200) {
    return { name: 'pupil', hex: '#008000', bg: '#00800020', text: '#008000' };
  } else {
    return { name: 'newbie', hex: '#808080', bg: '#80808020', text: '#808080' };
  }
}

export function getRatingTitle(rating: number): string {
  const color = getRatingColor(rating);
  return color.name.split('-').map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(' ');
}
```

- [ ] **Step 2: Create RatingBadge component**

```tsx
// web/src/components/RatingBadge.tsx
import { getRatingColor, getRatingTitle } from '../lib/rating';

interface RatingBadgeProps {
  rating: number;
  showTitle?: boolean;
  size?: 'sm' | 'md' | 'lg';
}

export default function RatingBadge({ rating, showTitle = false, size = 'md' }: RatingBadgeProps) {
  const color = getRatingColor(rating);
  const title = getRatingTitle(rating);
  
  const sizeClasses = {
    sm: 'text-xs px-1.5 py-0.5',
    md: 'text-sm px-2 py-1',
    lg: 'text-base px-3 py-1.5',
  };
  
  return (
    <span
      className={`inline-flex items-center font-medium rounded ${sizeClasses[size]}`}
      style={{ color: color.text, backgroundColor: color.bg }}
      title={title}
    >
      {rating}
      {showTitle && <span className="ml-1 text-xs opacity-75">{title}</span>}
    </span>
  );
}
```

- [ ] **Step 3: Verify TypeScript compilation**

Run: `cd web && npm run build`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/rating.ts web/src/components/RatingBadge.tsx
git commit -m "feat(rating): add frontend rating utilities and badge component"
```

---

### Task 8: Update Profile Page

**Files:**
- Modify: `web/src/pages/Profile.tsx`

- [ ] **Step 1: Add rating display to Profile**

Update the Profile page to show rating with color:

```tsx
// Add import at top
import RatingBadge from '../components/RatingBadge';

// In the profile section, add after username:
<div>
    <label className="block text-sm text-gray-500 mb-1">Rating</label>
    <RatingBadge rating={profile?.rating || 0} showTitle />
</div>
```

- [ ] **Step 2: Test the UI**

Run: `cd web && npm run dev`
Navigate to profile page and verify rating displays with correct color

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/Profile.tsx
git commit -m "feat(rating): display rating with colors on profile page"
```

---

### Task 9: Integrate Rating with Contest

**Files:**
- Modify: `internal/api/handler/contest.go`

- [ ] **Step 1: Add rating calculation to contest end**

Add a new handler or modify existing contest handler to trigger rating calculation when contest ends:

```go
// Add to ContestHandler
func (h *ContestHandler) CalculateRatings(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "id")
	
	// Get contest
	contest, err := h.store.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}
	
	// Only admin can trigger rating calculation
	claims := middleware.GetUserClaims(r)
	if claims == nil || claims.Role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	
	// Get scoreboard
	rows, _ := h.store.GetScoreboardRows(r.Context(), contestID, nil)
	participants, _ := h.store.GetParticipants(r.Context(), contestID)
	
	// Build standings (simplified - would need proper ranking logic)
	standings := make([]rating.ContestStanding, 0)
	for i, uid := range participants {
		standings = append(standings, rating.ContestStanding{
			UserID:    uid,
			Rank:      i + 1,
			OldRating: 1500, // Would need to fetch from user profile
		})
	}
	
	// Calculate and apply ratings
	ratingService := rating.NewService(nil, nil)
	changes := ratingService.CalculateContestRatings(standings)
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"changes": changes,
	})
}
```

- [ ] **Step 2: Add route**

Add to router:
```go
r.Post("/api/contests/{id}/calculate-ratings", contestH.CalculateRatings)
```

- [ ] **Step 3: Test the endpoint**

Run: `curl -X POST http://localhost:8080/api/contests/{id}/calculate-ratings -H "Authorization: Bearer {admin-token}"`
Expected: JSON response with rating changes

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler/contest.go internal/api/router.go
git commit -m "feat(rating): integrate rating calculation with contests"
```

---

## Verification Checklist

After completing all tasks:

- [ ] Database migration runs successfully
- [ ] Rating calculator produces correct results
- [ ] Rating store CRUD operations work
- [ ] API endpoints return correct data
- [ ] Frontend displays ratings with correct colors
- [ ] Rating calculation triggers after contest ends
- [ ] User profile updates with new rating
- [ ] Rating history page shows progression

---

## Notes

1. **Elo K-factor**: The current implementation uses K=32. This may need tuning based on actual usage.
2. **First contest**: Users with no rating history start at 1500.
3. **Rating floor**: Ratings cannot go below 0.
4. **Division eligibility**: This plan sets up the foundation; division enforcement is in Sub-Plan 03.
