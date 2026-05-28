# Problem Recommendations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete personalized Problem Recommendations dashboard (Progression, Weak Tags, and Hybrid) in both Go backend and React frontend.

**Architecture:** Extend store interfaces with a new recommendations query, define model structs, implement the HTTP route handler, and build a beautiful, tabbed frontend dashboard on `/practice`.

**Tech Stack:** Go 1.21+, PostgreSQL 16+, React 19, TypeScript, Tailwind CSS

---

## File Structure

- **Backend (Go):**
  - Create: `internal/model/recommendation.go` (new recommendation models)
  - Modify: `internal/store/interfaces.go` (add recommendation retrieval store interface)
  - Create: `internal/store/postgres/recommendations.go` (implement database recommendation query)
  - Create: `internal/api/handler/recommendation.go` (http API endpoint handler)
  - Modify: `internal/api/router.go` (add recommendations route)
  - Create: `internal/store/postgres/recommendations_test.go` (database query tests)
- **Frontend (React):**
  - Modify: `web/src/lib/api.ts` (add `api.recommendations.get` call)
  - Create: `web/src/pages/Practice.tsx` (the Practice tab dashboard)
  - Modify: `web/src/App.tsx` (add `/practice` route and Navbar link)

---

## Tasks

### Task 1: Recommendations Database Index & Migration

**Files:**
- Create: `internal/store/migrations/000017_problem_recommendations.up.sql`
- Create: `internal/store/migrations/000017_problem_recommendations.down.sql`

- [ ] **Step 1: Write UP migration**
Create the migration file to add an index on `submissions(user_id, status)` for fast tag error queries:

```sql
-- internal/store/migrations/000017_problem_recommendations.up.sql
CREATE INDEX IF NOT EXISTS idx_submissions_user_status_problem ON submissions(user_id, status, problem_id);
```

- [ ] **Step 2: Write DOWN migration**

```sql
-- internal/store/migrations/000017_problem_recommendations.down.sql
DROP INDEX IF EXISTS idx_submissions_user_status_problem;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: `migration up complete`

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000017_problem_recommendations.*
git commit -m "feat(recommendations): add db migration index for fast recommendation queries"
```

---

### Task 2: Models & Store Interface

**Files:**
- Create: `internal/model/recommendation.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Write model/recommendation.go**

```go
package model

type RecommendationsResponse struct {
	Progression []ProblemListItem       `json:"progression"`
	WeakTags    WeakTagsRecommendations `json:"weak_tags"`
	Hybrid      []ProblemListItem       `json:"hybrid"`
}

type WeakTagsRecommendations struct {
	Tags     []string          `json:"tags"`
	Problems []ProblemListItem `json:"problems"`
}
```

- [ ] **Step 2: Add Recommendations interface to store/interfaces.go**

Modify `internal/store/interfaces.go` to add `GetRecommendations` method on `ProblemStore` interface.

```go
type ProblemStore interface {
	Create(ctx context.Context, p *model.Problem) error
	GetByID(ctx context.Context, id string) (*model.Problem, error)
	GetBySlug(ctx context.Context, slug string) (*model.Problem, error)
	List(ctx context.Context, offset, limit int) ([]model.ProblemListItem, int, error)
	UpdateCounts(ctx context.Context, id string, addSubmission, addAccepted int) error
	Update(ctx context.Context, id string, p *model.Problem) error
	Delete(ctx context.Context, id string) error
	AddPermission(ctx context.Context, problemID, userID, accessLevel string) error
	RemovePermission(ctx context.Context, problemID, userID string) error
	GetPermissions(ctx context.Context, problemID string) ([]model.ProblemPermission, error)
	HasAccess(ctx context.Context, problemID, userID string, requiredLevels ...string) bool
	ListWithFilter(ctx context.Context, offset, limit int, difficulty string, tags []string, search string) ([]model.ProblemListItem, int, error)
	GetAllTags(ctx context.Context) ([]string, error)
	GetRecommendations(ctx context.Context, userID string, currentRating int) (*model.RecommendationsResponse, error)
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/model/... && go build ./internal/store/...`
Expected: Passes (with compiler warnings if interfaces are missing implementations, we will implement next).

- [ ] **Step 4: Commit**

```bash
git add internal/model/recommendation.go internal/store/interfaces.go
git commit -m "feat(recommendations): add recommendation model and extend ProblemStore interface"
```

---

### Task 3: Implement recommendations.go Store Query

**Files:**
- Create: `internal/store/postgres/recommendations.go`

- [ ] **Step 1: Write GetRecommendations implementation**

Create the file `internal/store/postgres/recommendations.go` containing the main recommendation logic:

```go
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	"github.com/lib/pq"
	"github.com/tahsinarafat/aioj/internal/model"
)

func (s *ProblemStore) GetRecommendations(ctx context.Context, userID string, currentRating int) (*model.RecommendationsResponse, error) {
	// 1. Map rating to target difficulty level
	var difficulty string
	if currentRating < 1400 {
		difficulty = "easy"
	} else if currentRating < 1900 {
		difficulty = "medium"
	} else {
		difficulty = "hard"
	}

	// 2. Fetch Progression Problems: Visible problems of matching difficulty that are unsolved by the user
	progQuery := `
		SELECT p.id, p.slug, p.title, p.difficulty, p.tags, p.submission_count, p.accepted_count, p.source
		FROM problems p
		WHERE p.visible = true AND p.difficulty = $1
		  AND p.id NOT IN (
		      SELECT DISTINCT problem_id FROM submissions WHERE user_id = $2 AND status = 'ac'
		  )
		ORDER BY p.accepted_count DESC, p.created_at DESC
		LIMIT 5`
	
	progRows, err := s.db.QueryContext(ctx, progQuery, difficulty, userID)
	if err != nil {
		return nil, fmt.Errorf("progression query: %w", err)
	}
	defer progRows.Close()

	var progression []model.ProblemListItem
	for progRows.Next() {
		var item model.ProblemListItem
		var tags []string
		if err := progRows.Scan(&item.ID, &item.Slug, &item.Title, &item.Difficulty, pq.Array(&tags),
			&item.SubmissionCount, &item.AcceptedCount, &item.Source); err != nil {
			return nil, err
		}
		item.Tags = tags
		progression = append(progression, item)
	}
	if progression == nil {
		progression = []model.ProblemListItem{}
	}

	// 3. Find Weak Tags: Identify top 2 tags with the highest non-AC submission counts
	weakTagsQuery := `
		SELECT t.tag, COUNT(*) as failed_count
		FROM submissions s
		JOIN problems p ON s.problem_id = p.id
		CROSS JOIN LATERAL unnest(p.tags) AS t(tag)
		WHERE s.user_id = $1 AND s.status != 'ac'
		  AND s.problem_id NOT IN (
		      SELECT DISTINCT problem_id FROM submissions WHERE user_id = $1 AND status = 'ac'
		  )
		GROUP BY t.tag
		ORDER BY failed_count DESC
		LIMIT 2`
	
	tagRows, err := s.db.QueryContext(ctx, weakTagsQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("weak tags count query: %w", err)
	}
	defer tagRows.Close()

	var weakTags []string
	for tagRows.Next() {
		var tag string
		var count int
		if err := tagRows.Scan(&tag, &count); err != nil {
			return nil, err
		}
		weakTags = append(weakTags, tag)
	}

	// Default fallbacks if no weak tags exist
	if len(weakTags) == 0 {
		weakTags = []string{"dp", "graphs"}
	} else if len(weakTags) == 1 {
		weakTags = append(weakTags, "greedy")
	}

	// 4. Fetch Weak-Tag problems: problems matching weak tags that are unsolved
	weakQuery := `
		SELECT p.id, p.slug, p.title, p.difficulty, p.tags, p.submission_count, p.accepted_count, p.source
		FROM problems p
		WHERE p.visible = true AND p.tags && $1
		  AND p.id NOT IN (
		      SELECT DISTINCT problem_id FROM submissions WHERE user_id = $2 AND status = 'ac'
		  )
		ORDER BY p.accepted_count DESC
		LIMIT 5`
	
	weakRows, err := s.db.QueryContext(ctx, weakQuery, pq.Array(weakTags), userID)
	if err != nil {
		return nil, fmt.Errorf("weak problems query: %w", err)
	}
	defer weakRows.Close()

	var weakProblems []model.ProblemListItem
	for weakRows.Next() {
		var item model.ProblemListItem
		var tags []string
		if err := weakRows.Scan(&item.ID, &item.Slug, &item.Title, &item.Difficulty, pq.Array(&tags),
			&item.SubmissionCount, &item.AcceptedCount, &item.Source); err != nil {
			return nil, err
		}
		item.Tags = tags
		weakProblems = append(weakProblems, item)
	}
	if weakProblems == nil {
		weakProblems = []model.ProblemListItem{}
	}

	// 5. Generate Hybrid List (Curated curriculum): 2 progression, 2 weak tags, 1 daily challenge
	var hybrid []model.ProblemListItem
	
	// Add up to 2 progression
	for i := 0; i < len(progression) && i < 2; i++ {
		hybrid = append(hybrid, progression[i])
	}
	
	// Add up to 2 weak tag problems
	for i := 0; i < len(weakProblems) && i < 2; i++ {
		// Avoid duplicates
		exists := false
		for _, h := range hybrid {
			if h.ID == weakProblems[i].ID {
				exists = true
				break
			}
		}
		if !exists {
			hybrid = append(hybrid, weakProblems[i])
		}
	}

	// Add 1 random visible unsolved problem as Daily Challenge
	dailyQuery := `
		SELECT p.id, p.slug, p.title, p.difficulty, p.tags, p.submission_count, p.accepted_count, p.source
		FROM problems p
		WHERE p.visible = true AND p.difficulty = $1
		  AND p.id NOT IN (
		      SELECT DISTINCT problem_id FROM submissions WHERE user_id = $2 AND status = 'ac'
		  )
		LIMIT 20`
	
	dailyRows, err := s.db.QueryContext(ctx, dailyQuery, difficulty, userID)
	if err == nil {
		defer dailyRows.Close()
		var candidates []model.ProblemListItem
		for dailyRows.Next() {
			var item model.ProblemListItem
			var tags []string
			if err := dailyRows.Scan(&item.ID, &item.Slug, &item.Title, &item.Difficulty, pq.Array(&tags),
				&item.SubmissionCount, &item.AcceptedCount, &item.Source); err == nil {
				item.Tags = tags
				candidates = append(candidates, item)
			}
		}
		if len(candidates) > 0 {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			selected := candidates[r.Intn(len(candidates))]
			
			// Avoid duplicate in hybrid list
			exists := false
			for _, h := range hybrid {
				if h.ID == selected.ID {
					exists = true
					break
				}
			}
			if !exists {
				hybrid = append(hybrid, selected)
			}
		}
	}

	if hybrid == nil {
		hybrid = []model.ProblemListItem{}
	}

	return &model.RecommendationsResponse{
		Progression: progression,
		WeakTags: model.WeakTagsRecommendations{
			Tags:     weakTags,
			Problems: weakProblems,
		},
		Hybrid:      hybrid,
	}, nil
}
```

- [ ] **Step 2: Compile backend**

Run: `go build ./internal/store/postgres`
Expected: `BUILD OK`

- [ ] **Step 3: Commit**

```bash
git add internal/store/postgres/recommendations.go
git commit -m "feat(recommendations): implement database query layer for personalized recommendations"
```

---

### Task 4: Store Integration Test

**Files:**
- Create: `internal/store/postgres/recommendations_test.go`

- [ ] **Step 1: Write recommendations_test.go**

```go
package postgres

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tahsinarafat/aioj/internal/model"
)

func TestGetRecommendations(t *testing.T) {
	// Connect to test db
	dsn := "postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// 1. Scaffold stores
	userStore := NewUserStore(db)
	probStore := NewProblemStore(db)

	// 2. Create test user
	userID := uuid.New().String()
	user := &model.User{
		ID:           userID,
		Username:     "rec_tester_" + uuid.New().String()[:8],
		Email:        "rec_tester_" + uuid.New().String()[:8] + "@gmail.com",
		PasswordHash: "xyz",
		Role:         "user",
	}
	if err := userStore.Create(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 3. Create test problems
	p1 := &model.Problem{
		ID:          uuid.New().String(),
		Slug:        "rec-prob-1-" + uuid.New().String()[:8],
		Title:       "Dynamic Programming easy",
		Description: "Easy DP problem",
		Difficulty:  "easy",
		Tags:        []string{"dp", "math"},
		Visible:     true,
		CreatedBy:   userID,
	}
	if err := probStore.Create(ctx, p1); err != nil {
		t.Fatalf("failed to create p1: %v", err)
	}

	p2 := &model.Problem{
		ID:          uuid.New().String(),
		Slug:        "rec-prob-2-" + uuid.New().String()[:8],
		Title:       "Graphs easy",
		Description: "Easy Graphs problem",
		Difficulty:  "easy",
		Tags:        []string{"graphs"},
		Visible:     true,
		CreatedBy:   userID,
	}
	if err := probStore.Create(ctx, p2); err != nil {
		t.Fatalf("failed to create p2: %v", err)
	}

	// 4. Test recommendations fetch
	resp, err := probStore.GetRecommendations(ctx, userID, 1200)
	if err != nil {
		t.Fatalf("GetRecommendations returned error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected recommendations response to be non-nil")
	}

	t.Logf("Found %d progression problems, %d weak tag problems", len(resp.Progression), len(resp.WeakTags.Problems))
}
```

- [ ] **Step 2: Run store tests**

Run: `go test -v ./internal/store/postgres/ -run TestGetRecommendations`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/store/postgres/recommendations_test.go
git commit -m "test(recommendations): add store-level integration test"
```

---

### Task 5: HTTP Handler & Router Route

**Files:**
- Create: `internal/api/handler/recommendation.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Write handler/recommendation.go**

Create `internal/api/handler/recommendation.go` to handle recommendations requests:

```go
package handler

import (
	"net/http"
	"strconv"

	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store"
)

type RecommendationHandler struct {
	probStore  store.ProblemStore
	ratingStore store.RatingStore
}

func NewRecommendationHandler(p store.ProblemStore, r store.RatingStore) *RecommendationHandler {
	return &RecommendationHandler{probStore: p, ratingStore: r}
}

func (h *RecommendationHandler) GetRecommendations(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Fetch latest user rating (default to 1200 if not found)
	rating := 1200
	latest, err := h.ratingStore.GetLatestByUser(r.Context(), claims.UserID)
	if err == nil && latest != nil {
		rating = latest.NewRating
	}

	// Override rating with query parameter if present (useful for setter/admin testing)
	if qRating := r.URL.Query().Get("rating"); qRating != "" {
		if val, err := strconv.Atoi(qRating); err == nil {
			rating = val
		}
	}

	resp, err := h.probStore.GetRecommendations(r.Context(), claims.UserID, rating)
	if err != nil {
		http.Error(w, "failed to get recommendations: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Register route in router.go**

Modify `internal/api/router.go` to wire up the new handler under `/api/recommendations`:

```go
// Add import if needed or reference handler.NewRecommendationHandler
// Locate router setup and wire:

// Under authenticated route group (e.g. inside `r.Group(func(r chi.Router) { r.Use(middleware.AuthMiddleware(jwtManager)) ... })`):
r.Get("/api/recommendations", recommendationH.GetRecommendations)
```

We need to check where routes are wired in `cmd/aioj/main.go` and `internal/api/router.go`. Let's inspect `internal/api/router.go` to see the route signatures. Let's read `internal/api/router.go`.

- [ ] **Step 3: Modify `cmd/aioj/main.go` to inject recommendations handler**

We will read `cmd/aioj/main.go` and add the `NewRecommendationHandler` instantiation, passing it to `NewRouter`.

- [ ] **Step 4: Verify Go build**

Run: `go build ./...`
Expected: `BUILD OK`

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/recommendation.go internal/api/router.go cmd/aioj/main.go
git commit -m "feat(recommendations): add recommendations HTTP API handler and router registration"
```

---

### Task 6: Frontend API Integration

**Files:**
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add recommendations API client methods**

Read `web/src/lib/api.ts` and locate the `api` export constant. Add the `recommendations` object:

```typescript
recommendations: {
    get: (rating?: number) =>
        request<{
            progression: any[];
            weak_tags: { tags: string[]; problems: any[] };
            hybrid: any[];
        }>(rating !== undefined ? `/recommendations?rating=${rating}` : '/recommendations'),
},
```

- [ ] **Step 2: Build frontend to verify compilation**

Run: `cd web && npm run build`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "feat(recommendations): add recommendations methods to frontend API client"
```

---

### Task 7: Practice Dashboard Page

**Files:**
- Create: `web/src/pages/Practice.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Write Practice.tsx**

Create a clean, dashboard practice view with sections for daily recommended, progression, and targeted tags:

```tsx
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../lib/api'

export default function Practice() {
    const [rec, setRec] = useState<any>(null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState('')
    const [activeTab, setActiveTab] = useState<'hybrid' | 'progression' | 'weak'>('hybrid')

    useEffect(() => {
        api.recommendations.get()
            .then(data => {
                setRec(data)
                setLoading(false)
            })
            .catch(err => {
                setError(err.message || 'Failed to load recommendations')
                setLoading(false)
            })
    }, [])

    if (loading) return <div className="text-center py-12 text-gray-500">Loading recommendations...</div>
    if (error) return <div className="max-w-xl mx-auto bg-red-50 text-red-700 p-4 rounded-md border border-red-100 my-6">{error}</div>

    const problems = activeTab === 'hybrid' ? rec?.hybrid : activeTab === 'progression' ? rec?.progression : rec?.weak_tags?.problems

    return (
        <div className="max-w-4xl mx-auto">
            <header className="mb-8">
                <h1 className="text-3xl font-extrabold tracking-tight text-gray-900">Personalized Practice</h1>
                <p className="mt-2 text-gray-600">Smart problem recommendations tailored to your rating and weak areas.</p>
            </header>

            {/* Profile summary banner */}
            <div className="bg-gradient-to-r from-blue-50 to-indigo-50 rounded-xl p-6 border border-blue-100 mb-8 flex justify-between items-center">
                <div>
                    <h3 className="font-semibold text-blue-900 text-lg">Practice Mode Active</h3>
                    <p className="text-sm text-blue-700 mt-1">Analyzing your contest ratings and submissions to offer smart challenges.</p>
                </div>
                <div className="bg-blue-600 text-white font-mono px-4 py-2 rounded-lg text-sm font-bold shadow-sm">
                    Level Up
                </div>
            </div>

            {/* Tabs selector */}
            <div className="border-b border-gray-200 mb-6">
                <nav className="flex gap-6">
                    <button onClick={() => setActiveTab('hybrid')}
                        className={`pb-4 text-sm font-medium ${activeTab === 'hybrid' ? 'border-b-2 border-blue-600 text-blue-600 font-semibold' : 'text-gray-500 hover:text-gray-700'}`}>
                        Daily Diet (Hybrid)
                    </button>
                    <button onClick={() => setActiveTab('progression')}
                        className={`pb-4 text-sm font-medium ${activeTab === 'progression' ? 'border-b-2 border-blue-600 text-blue-600 font-semibold' : 'text-gray-500 hover:text-gray-700'}`}>
                        Progression Ladder
                    </button>
                    <button onClick={() => setActiveTab('weak')}
                        className={`pb-4 text-sm font-medium ${activeTab === 'weak' ? 'border-b-2 border-blue-600 text-blue-600 font-semibold' : 'text-gray-500 hover:text-gray-700'}`}>
                        Targeted Tags Practice
                    </button>
                </nav>
            </div>

            {/* Recommendations Content */}
            <div className="space-y-4">
                {problems && problems.length > 0 ? (
                    problems.map((p: any) => (
                        <div key={p.id} className="bg-white border border-gray-200 rounded-lg p-5 hover:border-blue-300 transition-colors shadow-sm flex items-center justify-between">
                            <div className="space-y-1">
                                <Link to={`/problems/${p.slug}`} className="text-lg font-bold text-gray-900 hover:text-blue-600 transition-colors">
                                    {p.title}
                                </Link>
                                <div className="flex flex-wrap gap-2 pt-1 items-center">
                                    <span className={`px-2 py-0.5 rounded text-xs font-semibold uppercase ${
                                        p.difficulty === 'easy' ? 'bg-green-50 text-green-700 border border-green-100' :
                                        p.difficulty === 'medium' ? 'bg-yellow-50 text-yellow-700 border border-yellow-100' :
                                        'bg-red-50 text-red-700 border border-red-100'
                                    }`}>
                                        {p.difficulty}
                                    </span>
                                    {p.tags && p.tags.map((t: string) => (
                                        <span key={t} className="bg-gray-50 border border-gray-100 text-gray-600 px-2 py-0.5 rounded text-xs">
                                            {t}
                                        </span>
                                    ))}
                                </div>
                            </div>
                            <div className="text-right">
                                <span className="text-sm font-medium text-gray-500">Acceptance Rate</span>
                                <p className="text-sm font-bold text-gray-900">
                                    {p.submission_count > 0 ? `${Math.round((p.accepted_count / p.submission_count) * 100)}%` : '0%'}
                                </p>
                            </div>
                        </div>
                    ))
                ) : (
                    <div className="text-center py-12 text-gray-400 bg-gray-50 border border-dashed rounded-lg">
                        {activeTab === 'weak' ? 'Keep submitting! Submit more problems to see your weak topic areas analyzed.' : 'Excellent work! You have cleared all recommended problems in this level.'}
                    </div>
                )}

                {activeTab === 'weak' && rec?.weak_tags?.tags?.length > 0 && (
                    <div className="bg-amber-50 border border-amber-200 rounded-lg p-4 text-sm text-amber-800 mt-6">
                        🎯 Recommending problems on your weakest tags: <span className="font-bold">{rec.weak_tags.tags.join(', ')}</span>
                    </div>
                )}
            </div>
        </div>
    )
}
```

- [ ] **Step 2: Update App.tsx with route and navbar link**

Inject Practice page route:
`import Practice from './pages/Practice'`

Add under the main layout routes:
`<Route path="/practice" element={<Practice />} />`

Add Navbar link under logged-in views:
`<Link to="/practice" className="text-sm text-gray-600 hover:text-black">Practice</Link>`

- [ ] **Step 3: Run full vite builds**

Run: `cd web && npm run build`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/Practice.tsx web/src/App.tsx
git commit -m "feat(recommendations): add frontend practice dashboard UI"
```

---

## Verification & Final E2E Run

- [ ] **Step 1:** Run database migration: `make migrate-up`
- [ ] **Step 2:** Build and test backend: `go test ./...`
- [ ] **Step 3:** Rebuild Docker images: `docker compose build --no-cache`
- [ ] **Step 4:** Deploy containers: `docker compose up -d`
- [ ] **Step 5:** Visit `http://localhost/practice` to verify UI loading, correct tags analysis, progression ladder filtering, and AC submit integration.
