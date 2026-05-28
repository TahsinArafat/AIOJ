# Sub-Plan 06: Gym/Training

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a Gym section for community-written training contests with difficulty filters and categories.

**Architecture:** Add `gym_contests` table, gym service, frontend gym browsing and filtering.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Create
- `internal/model/gym.go` - Gym models
- `internal/store/postgres/gym.go` - Gym store
- `internal/api/handler/gym.go` - Gym handler

### Backend Files to Modify
- `internal/store/interfaces.go` - Add GymStore interface
- `internal/api/router.go` - Add gym routes

### Frontend Files to Create
- `web/src/pages/GymList.tsx` - Gym contest listing
- `web/src/pages/GymDetail.tsx` - Gym contest detail
- `web/src/pages/GymCreate.tsx` - Create gym contest

### Frontend Files to Modify
- `web/src/App.tsx` - Add gym routes
- `web/src/lib/api.ts` - Add gym API calls

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000009_gym.up.sql`
- Create: `internal/store/migrations/000009_gym.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000009_gym.up.sql

CREATE TABLE gym_contests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    difficulty_rating INTEGER CHECK (difficulty_rating >= 800 AND difficulty_rating <= 3500),
    category VARCHAR(64) NOT NULL DEFAULT 'general',
    country VARCHAR(64),
    season VARCHAR(32),
    description TEXT NOT NULL DEFAULT '',
    is_public BOOLEAN NOT NULL DEFAULT true,
    solve_count INTEGER NOT NULL DEFAULT 0,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE gym_solves (
    gym_id UUID NOT NULL REFERENCES gym_contests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    solved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (gym_id, user_id)
);

CREATE INDEX idx_gym_category ON gym_contests(category);
CREATE INDEX idx_gym_difficulty ON gym_contests(difficulty_rating);
CREATE INDEX idx_gym_public ON gym_contests(is_public) WHERE is_public = true;
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000009_gym.down.sql

DROP TABLE IF EXISTS gym_solves;
DROP TABLE IF EXISTS gym_contests;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: Migration applied successfully

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000009_gym.*
git commit -m "feat(gym): add gym database migration"
```

---

### Task 2: Gym Models

**Files:**
- Create: `internal/model/gym.go`

- [ ] **Step 1: Create gym models**

```go
// internal/model/gym.go
package model

import "time"

const (
	GymCategoryGeneral       = "general"
	GymCategoryICPC          = "icpc"
	GymCategoryIOI           = "ioi"
	GymCategoryEducational   = "educational"
	GymCategoryRegional      = "regional"
	GymCategoryNational      = "national"
	GymCategoryOpen          = "open"
)

var GymCategories = []string{
	GymCategoryGeneral,
	GymCategoryICPC,
	GymCategoryIOI,
	GymCategoryEducational,
	GymCategoryRegional,
	GymCategoryNational,
	GymCategoryOpen,
}

type GymContest struct {
	ID              string    `json:"id"`
	ContestID       string    `json:"contest_id"`
	ContestTitle    string    `json:"contest_title,omitempty"`
	DifficultyRating *int     `json:"difficulty_rating,omitempty"`
	Category        string    `json:"category"`
	Country         string    `json:"country,omitempty"`
	Season          string    `json:"season,omitempty"`
	Description     string    `json:"description"`
	IsPublic        bool      `json:"is_public"`
	SolveCount      int       `json:"solve_count"`
	CreatedBy       string    `json:"created_by"`
	CreatorName     string    `json:"creator_name,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type GymSolve struct {
	GymID    string    `json:"gym_id"`
	UserID   string    `json:"user_id"`
	SolvedAt time.Time `json:"solved_at"`
}

type CreateGymRequest struct {
	ContestID        string `json:"contest_id"`
	DifficultyRating *int   `json:"difficulty_rating,omitempty"`
	Category         string `json:"category"`
	Country          string `json:"country,omitempty"`
	Season           string `json:"season,omitempty"`
	Description      string `json:"description"`
	IsPublic         bool   `json:"is_public"`
}

type GymFilter struct {
	Category   string
	MinRating  *int
	MaxRating  *int
	Country    string
	Search     string
	SolvedByUser *string
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/gym.go
git commit -m "feat(gym): add gym models"
```

---

### Task 3: Gym Store

**Files:**
- Create: `internal/store/postgres/gym.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Add GymStore interface**

Add to `internal/store/interfaces.go`:

```go
type GymStore interface {
	Create(ctx context.Context, g *model.GymContest) error
	GetByID(ctx context.Context, id string) (*model.GymContest, error)
	List(ctx context.Context, offset, limit int, filter model.GymFilter) ([]model.GymContest, int, error)
	Update(ctx context.Context, id string, g *model.GymContest) error
	Delete(ctx context.Context, id string) error
	
	MarkSolved(ctx context.Context, gymID, userID string) error
	IsSolved(ctx context.Context, gymID, userID string) (bool, error)
	GetSolveCount(ctx context.Context, gymID string) (int, error)
}
```

- [ ] **Step 2: Implement gym store**

```go
// internal/store/postgres/gym.go
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/tahsinarafat/aioj/internal/model"
)

type GymStore struct {
	db *sql.DB
}

func NewGymStore(db *sql.DB) *GymStore {
	return &GymStore{db: db}
}

func (s *GymStore) Create(ctx context.Context, g *model.GymContest) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO gym_contests (contest_id, difficulty_rating, category, country, season, description, is_public, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, created_at`,
		g.ContestID, g.DifficultyRating, g.Category, g.Country, g.Season, g.Description, g.IsPublic, g.CreatedBy,
	).Scan(&g.ID, &g.CreatedAt)
}

func (s *GymStore) GetByID(ctx context.Context, id string) (*model.GymContest, error) {
	var g model.GymContest
	err := s.db.QueryRowContext(ctx,
		`SELECT g.id, g.contest_id, c.title, g.difficulty_rating, g.category, g.country, g.season,
		        g.description, g.is_public, g.solve_count, g.created_by, u.username, g.created_at
		 FROM gym_contests g
		 JOIN contests c ON g.contest_id = c.id
		 JOIN users u ON g.created_by = u.id
		 WHERE g.id = $1`,
		id).Scan(&g.ID, &g.ContestID, &g.ContestTitle, &g.DifficultyRating, &g.Category,
		&g.Country, &g.Season, &g.Description, &g.IsPublic, &g.SolveCount,
		&g.CreatedBy, &g.CreatorName, &g.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *GymStore) List(ctx context.Context, offset, limit int, filter model.GymFilter) ([]model.GymContest, int, error) {
	where := []string{"g.is_public = true"}
	args := []interface{}{}
	argIdx := 1
	
	if filter.Category != "" {
		where = append(where, fmt.Sprintf("g.category = $%d", argIdx))
		args = append(args, filter.Category)
		argIdx++
	}
	
	if filter.MinRating != nil {
		where = append(where, fmt.Sprintf("g.difficulty_rating >= $%d", argIdx))
		args = append(args, *filter.MinRating)
		argIdx++
	}
	
	if filter.MaxRating != nil {
		where = append(where, fmt.Sprintf("g.difficulty_rating <= $%d", argIdx))
		args = append(args, *filter.MaxRating)
		argIdx++
	}
	
	if filter.Country != "" {
		where = append(where, fmt.Sprintf("g.country ILIKE $%d", argIdx))
		args = append(args, "%"+filter.Country+"%")
		argIdx++
	}
	
	if filter.Search != "" {
		where = append(where, fmt.Sprintf("(c.title ILIKE $%d OR g.description ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}
	
	whereClause := strings.Join(where, " AND ")
	
	// Count
	var total int
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM gym_contests g JOIN contests c ON g.contest_id = c.id WHERE "+whereClause,
		args...).Scan(&total)
	
	// Query
	query := fmt.Sprintf(`SELECT g.id, g.contest_id, c.title, g.difficulty_rating, g.category, g.country,
	                              g.season, g.description, g.is_public, g.solve_count, g.created_by, u.username, g.created_at
	                      FROM gym_contests g
	                      JOIN contests c ON g.contest_id = c.id
	                      JOIN users u ON g.created_by = u.id
	                      WHERE %s
	                      ORDER BY g.created_at DESC OFFSET $%d LIMIT $%d`,
		whereClause, argIdx, argIdx+1)
	args = append(args, offset, limit)
	
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var items []model.GymContest
	for rows.Next() {
		var g model.GymContest
		if err := rows.Scan(&g.ID, &g.ContestID, &g.ContestTitle, &g.DifficultyRating, &g.Category,
			&g.Country, &g.Season, &g.Description, &g.IsPublic, &g.SolveCount,
			&g.CreatedBy, &g.CreatorName, &g.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, g)
	}
	if items == nil {
		items = []model.GymContest{}
	}
	return items, total, nil
}

func (s *GymStore) MarkSolved(ctx context.Context, gymID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO gym_solves (gym_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		gymID, userID)
	if err == nil {
		s.db.ExecContext(ctx, "UPDATE gym_contests SET solve_count = solve_count + 1 WHERE id = $1", gymID)
	}
	return err
}

func (s *GymStore) IsSolved(ctx context.Context, gymID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM gym_solves WHERE gym_id = $1 AND user_id = $2)",
		gymID, userID).Scan(&exists)
	return exists, err
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/gym.go
git commit -m "feat(gym): add gym store"
```

---

### Task 4: Gym Handler

**Files:**
- Create: `internal/api/handler/gym.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create gym handler**

```go
// internal/api/handler/gym.go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type GymHandler struct {
	store *postgres.GymStore
}

func NewGymHandler(s *postgres.GymStore) *GymHandler {
	return &GymHandler{store: s}
}

func (h *GymHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	var req model.CreateGymRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	if req.Category == "" {
		req.Category = model.GymCategoryGeneral
	}
	
	g := &model.GymContest{
		ContestID:        req.ContestID,
		DifficultyRating: req.DifficultyRating,
		Category:         req.Category,
		Country:          req.Country,
		Season:           req.Season,
		Description:      req.Description,
		IsPublic:         req.IsPublic,
		CreatedBy:        claims.UserID,
	}
	
	if err := h.store.Create(r.Context(), g); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusCreated, g)
}

func (h *GymHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	
	filter := model.GymFilter{
		Category: r.URL.Query().Get("category"),
		Country:  r.URL.Query().Get("country"),
		Search:   r.URL.Query().Get("search"),
	}
	
	if minStr := r.URL.Query().Get("min_rating"); minStr != "" {
		min, _ := strconv.Atoi(minStr)
		filter.MinRating = &min
	}
	if maxStr := r.URL.Query().Get("max_rating"); maxStr != "" {
		max, _ := strconv.Atoi(maxStr)
		filter.MaxRating = &max
	}
	
	items, total, _ := h.store.List(r.Context(), offset, limit, filter)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *GymHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	g, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if g == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	
	respondJSON(w, http.StatusOK, g)
}

func (h *GymHandler) MarkSolved(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	id := chi.URLParam(r, "id")
	if err := h.store.MarkSolved(r.Context(), id, claims.UserID); err != nil {
		http.Error(w, "failed to mark solved", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]string{"status": "solved"})
}
```

- [ ] **Step 2: Add routes**

Add to `internal/api/router.go`:

```go
r.Route("/api/gym", func(r chi.Router) {
	r.Get("/", gymH.List)
	r.Get("/{id}", gymH.GetByID)
	
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/", gymH.Create)
		r.Post("/{id}/solve", gymH.MarkSolved)
	})
})
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/gym.go internal/api/router.go
git commit -m "feat(gym): add gym API endpoints"
```

---

### Task 5: Frontend Gym Pages

**Files:**
- Create: `web/src/pages/GymList.tsx`
- Create: `web/src/pages/GymDetail.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add gym API calls**

```typescript
gym: {
    list: (offset = 0, limit = 20, filters?: {
        category?: string;
        min_rating?: number;
        max_rating?: number;
        search?: string;
    }) => {
        let url = `/gym?offset=${offset}&limit=${limit}`;
        if (filters?.category) url += `&category=${filters.category}`;
        if (filters?.min_rating) url += `&min_rating=${filters.min_rating}`;
        if (filters?.max_rating) url += `&max_rating=${filters.max_rating}`;
        if (filters?.search) url += `&search=${encodeURIComponent(filters.search)}`;
        return request<{ data: any[]; total: number }>(url);
    },
    get: (id: string) => request<any>(`/gym/${id}`),
    create: (data: any) => request<any>('/gym', { method: 'POST', body: JSON.stringify(data) }),
    markSolved: (id: string) => request(`/gym/${id}/solve`, { method: 'POST' }),
},
```

- [ ] **Step 2: Create GymList page**

```tsx
// web/src/pages/GymList.tsx
import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../lib/api';

const CATEGORIES = [
  { value: '', label: 'All Categories' },
  { value: 'general', label: 'General' },
  { value: 'icpc', label: 'ICPC' },
  { value: 'ioi', label: 'IOI' },
  { value: 'educational', label: 'Educational' },
  { value: 'regional', label: 'Regional' },
  { value: 'national', label: 'National' },
  { value: 'open', label: 'Open' },
];

export default function GymList() {
  const [gyms, setGyms] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [category, setCategory] = useState('');
  const [search, setSearch] = useState('');
  const limit = 20;

  useEffect(() => {
    api.gym.list(offset, limit, { category, search }).then(d => {
      setGyms(d.data || []);
      setTotal(d.total || 0);
    }).catch(console.error);
  }, [offset, category, search]);

  const getDifficultyColor = (rating?: number) => {
    if (!rating) return 'text-gray-500';
    if (rating < 1200) return 'text-green-600';
    if (rating < 1600) return 'text-blue-600';
    if (rating < 2000) return 'text-purple-600';
    if (rating < 2400) return 'text-orange-600';
    return 'text-red-600';
  };

  return (
    <div>
      <h1 className="text-2xl font-bold mb-4">Gym</h1>
      <p className="text-gray-600 mb-6">Practice with community-curated contests</p>

      {/* Filters */}
      <div className="flex gap-4 mb-6">
        <select
          value={category}
          onChange={(e) => { setCategory(e.target.value); setOffset(0); }}
          className="border rounded px-3 py-2"
        >
          {CATEGORIES.map(c => (
            <option key={c.value} value={c.value}>{c.label}</option>
          ))}
        </select>
        <input
          type="text"
          value={search}
          onChange={(e) => { setSearch(e.target.value); setOffset(0); }}
          placeholder="Search gym..."
          className="border rounded px-3 py-2 flex-1"
        />
      </div>

      {/* Gym List */}
      <div className="grid gap-4">
        {gyms.map(g => (
          <Link key={g.id} to={`/gym/${g.id}`} className="block">
            <div className="border rounded-lg p-4 hover:bg-gray-50">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="font-semibold">{g.contest_title}</h3>
                  <p className="text-sm text-gray-600 mt-1">{g.description}</p>
                  <div className="flex gap-4 mt-2 text-sm text-gray-500">
                    <span className="capitalize">{g.category}</span>
                    {g.country && <span>{g.country}</span>}
                    <span>{g.solve_count} solved</span>
                  </div>
                </div>
                {g.difficulty_rating && (
                  <span className={`font-mono font-bold ${getDifficultyColor(g.difficulty_rating)}`}>
                    {g.difficulty_rating}
                  </span>
                )}
              </div>
            </div>
          </Link>
        ))}
      </div>

      {gyms.length === 0 && (
        <p className="text-center text-gray-500 py-10">No gym contests found.</p>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Create GymDetail page**

```tsx
// web/src/pages/GymDetail.tsx
import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { api, getAccessToken } from '../lib/api';

export default function GymDetail() {
  const { id } = useParams<{ id: string }>();
  const [gym, setGym] = useState<any>(null);
  const [solved, setSolved] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!id) return;
    api.gym.get(id).then(setGym).catch(() => {}).finally(() => setLoading(false));
  }, [id]);

  const handleMarkSolved = async () => {
    if (!id) return;
    try {
      await api.gym.markSolved(id);
      setSolved(true);
    } catch (e: any) {
      alert('Failed: ' + e.message);
    }
  };

  if (loading) return <div>Loading...</div>;
  if (!gym) return <div>Gym contest not found</div>;

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{gym.contest_title}</h1>
        <p className="text-gray-600 mt-2">{gym.description}</p>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-gray-50 p-4 rounded-lg">
          <p className="text-sm text-gray-500">Category</p>
          <p className="font-medium capitalize">{gym.category}</p>
        </div>
        {gym.difficulty_rating && (
          <div className="bg-gray-50 p-4 rounded-lg">
            <p className="text-sm text-gray-500">Difficulty</p>
            <p className="font-medium">{gym.difficulty_rating}</p>
          </div>
        )}
        <div className="bg-gray-50 p-4 rounded-lg">
          <p className="text-sm text-gray-500">Solved By</p>
          <p className="font-medium">{gym.solve_count} users</p>
        </div>
        {gym.country && (
          <div className="bg-gray-50 p-4 rounded-lg">
            <p className="text-sm text-gray-500">Country</p>
            <p className="font-medium">{gym.country}</p>
          </div>
        )}
      </div>

      <div className="flex gap-4">
        <Link
          to={`/contests/${gym.contest_id}`}
          className="bg-blue-600 text-white px-6 py-2 rounded hover:bg-blue-700"
        >
          Enter Contest
        </Link>
        {getAccessToken() && !solved && (
          <button
            onClick={handleMarkSolved}
            className="bg-green-600 text-white px-6 py-2 rounded hover:bg-green-700"
          >
            Mark as Solved
          </button>
        )}
        {solved && (
          <span className="flex items-center text-green-600 font-medium">
            ✓ Solved
          </span>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Add routes**

```tsx
<Route path="/gym" element={<GymList />} />
<Route path="/gym/:id" element={<GymDetail />} />
```

- [ ] **Step 5: Add to Navbar**

```tsx
<Link to="/gym" className="text-sm text-gray-600 hover:text-black transition-colors">Gym</Link>
```

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/GymList.tsx web/src/pages/GymDetail.tsx web/src/App.tsx web/src/lib/api.ts
git commit -m "feat(gym): add gym frontend pages"
```

---

## Verification Checklist

After completing all tasks:

- [ ] Database migration runs successfully
- [ ] Gym contests can be created
- [ ] Gym list filters work correctly
- [ ] Difficulty rating displays with color
- [ ] Mark as solved works
- [ ] Solve count updates correctly
- [ ] Category filter works
- [ ] Search filter works

---

## Notes

1. **Categories**: ICPC, IOI, Educational, Regional, National, Open
2. **Difficulty rating**: 800-3500 (Codeforces style)
3. **Solve tracking**: Users can mark gym contests as solved
4. **Public/Private**: Gym contests can be public or private
