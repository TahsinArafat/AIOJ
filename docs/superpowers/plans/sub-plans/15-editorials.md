# Sub-Plan 15: Editorials

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow official and community editorials with solutions for problems.

**Architecture:** Add `editorials` table, editorial service, frontend editorial display.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Create
- `internal/store/postgres/editorials.go` - Editorial store
- `internal/api/handler/editorial.go` - Editorial handler

### Backend Files to Modify
- `internal/store/interfaces.go` - Add EditorialStore interface
- `internal/api/router.go` - Add editorial routes

### Frontend Files to Create
- `web/src/pages/EditorialList.tsx` - Editorial listing
- `web/src/pages/EditorialDetail.tsx` - Editorial detail
- `web/src/pages/EditorialCreate.tsx` - Create editorial

### Frontend Files to Modify
- `web/src/pages/ProblemDetail.tsx` - Add editorial link
- `web/src/App.tsx` - Add editorial routes
- `web/src/lib/api.ts` - Add editorial API calls

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000013_editorials.up.sql`
- Create: `internal/store/migrations/000013_editorials.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000013_editorials.up.sql

CREATE TABLE editorials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    contest_id UUID REFERENCES contests(id),
    user_id UUID NOT NULL REFERENCES users(id),
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    solution_code TEXT,
    solution_language VARCHAR(64),
    approach TEXT,
    time_complexity VARCHAR(64),
    space_complexity VARCHAR(64),
    is_official BOOLEAN NOT NULL DEFAULT false,
    upvotes INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_editorials_problem ON editorials(problem_id);
CREATE INDEX idx_editorials_contest ON editorials(contest_id) WHERE contest_id IS NOT NULL;
CREATE INDEX idx_editorials_official ON editorials(is_official) WHERE is_official = true;
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000013_editorials.down.sql

DROP TABLE IF EXISTS editorials;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000013_editorials.*
git commit -m "feat(editorial): add editorials database migration"
```

---

### Task 2: Editorial Models and Store

**Files:**
- Create: `internal/model/editorial.go`
- Create: `internal/store/postgres/editorials.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Create editorial models**

```go
// internal/model/editorial.go
package model

import "time"

type Editorial struct {
	ID               string    `json:"id"`
	ProblemID        string    `json:"problem_id"`
	ProblemTitle     string    `json:"problem_title,omitempty"`
	ContestID        *string   `json:"contest_id,omitempty"`
	UserID           string    `json:"user_id"`
	Username         string    `json:"username,omitempty"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	SolutionCode     string    `json:"solution_code,omitempty"`
	SolutionLanguage string    `json:"solution_language,omitempty"`
	Approach         string    `json:"approach,omitempty"`
	TimeComplexity   string    `json:"time_complexity,omitempty"`
	SpaceComplexity  string    `json:"space_complexity,omitempty"`
	IsOfficial       bool      `json:"is_official"`
	Upvotes          int       `json:"upvotes"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CreateEditorialRequest struct {
	ProblemID        string `json:"problem_id"`
	ContestID        string `json:"contest_id,omitempty"`
	Title            string `json:"title"`
	Content          string `json:"content"`
	SolutionCode     string `json:"solution_code,omitempty"`
	SolutionLanguage string `json:"solution_language,omitempty"`
	Approach         string `json:"approach,omitempty"`
	TimeComplexity   string `json:"time_complexity,omitempty"`
	SpaceComplexity  string `json:"space_complexity,omitempty"`
}
```

- [ ] **Step 2: Add EditorialStore interface**

```go
type EditorialStore interface {
	Create(ctx context.Context, e *model.Editorial) error
	GetByID(ctx context.Context, id string) (*model.Editorial, error)
	GetByProblem(ctx context.Context, problemID string) ([]model.Editorial, error)
	List(ctx context.Context, offset, limit int) ([]model.Editorial, int, error)
	Update(ctx context.Context, id string, e *model.Editorial) error
	Delete(ctx context.Context, id string) error
}
```

- [ ] **Step 3: Implement editorial store**

```go
// internal/store/postgres/editorials.go
package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type EditorialStore struct {
	db *sql.DB
}

func NewEditorialStore(db *sql.DB) *EditorialStore {
	return &EditorialStore{db: db}
}

func (s *EditorialStore) Create(ctx context.Context, e *model.Editorial) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO editorials (problem_id, contest_id, user_id, title, content, solution_code, 
		                         solution_language, approach, time_complexity, space_complexity, is_official)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id, created_at, updated_at`,
		e.ProblemID, e.ContestID, e.UserID, e.Title, e.Content, e.SolutionCode,
		e.SolutionLanguage, e.Approach, e.TimeComplexity, e.SpaceComplexity, e.IsOfficial,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

func (s *EditorialStore) GetByID(ctx context.Context, id string) (*model.Editorial, error) {
	var e model.Editorial
	err := s.db.QueryRowContext(ctx,
		`SELECT e.id, e.problem_id, p.title, e.contest_id, e.user_id, u.username, e.title, e.content,
		        e.solution_code, e.solution_language, e.approach, e.time_complexity, e.space_complexity,
		        e.is_official, e.upvotes, e.created_at, e.updated_at
		 FROM editorials e
		 JOIN problems p ON e.problem_id = p.id
		 JOIN users u ON e.user_id = u.id
		 WHERE e.id = $1`,
		id).Scan(&e.ID, &e.ProblemID, &e.ProblemTitle, &e.ContestID, &e.UserID, &e.Username,
		&e.Title, &e.Content, &e.SolutionCode, &e.SolutionLanguage, &e.Approach,
		&e.TimeComplexity, &e.SpaceComplexity, &e.IsOfficial, &e.Upvotes, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *EditorialStore) GetByProblem(ctx context.Context, problemID string) ([]model.Editorial, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, e.problem_id, e.user_id, u.username, e.title, e.approach, 
		        e.time_complexity, e.is_official, e.upvotes, e.created_at
		 FROM editorials e JOIN users u ON e.user_id = u.id
		 WHERE e.problem_id = $1 ORDER BY e.is_official DESC, e.upvotes DESC`,
		problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var editorials []model.Editorial
	for rows.Next() {
		var e model.Editorial
		if err := rows.Scan(&e.ID, &e.ProblemID, &e.UserID, &e.Username, &e.Title,
			&e.Approach, &e.TimeComplexity, &e.IsOfficial, &e.Upvotes, &e.CreatedAt); err != nil {
			return nil, err
		}
		editorials = append(editorials, e)
	}
	if editorials == nil {
		editorials = []model.Editorial{}
	}
	return editorials, nil
}

func (s *EditorialStore) List(ctx context.Context, offset, limit int) ([]model.Editorial, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM editorials").Scan(&total)
	
	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, e.problem_id, p.title, e.user_id, u.username, e.title, e.is_official, e.upvotes, e.created_at
		 FROM editorials e
		 JOIN problems p ON e.problem_id = p.id
		 JOIN users u ON e.user_id = u.id
		 ORDER BY e.created_at DESC OFFSET $1 LIMIT $2`,
		offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var items []model.Editorial
	for rows.Next() {
		var e model.Editorial
		if err := rows.Scan(&e.ID, &e.ProblemID, &e.ProblemTitle, &e.UserID, &e.Username,
			&e.Title, &e.IsOfficial, &e.Upvotes, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, e)
	}
	if items == nil {
		items = []model.Editorial{}
	}
	return items, total, nil
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/model/editorial.go internal/store/interfaces.go internal/store/postgres/editorials.go
git commit -m "feat(editorial): add editorial models and store"
```

---

### Task 3: Editorial Handler and Frontend

**Files:**
- Create: `internal/api/handler/editorial.go`
- Modify: `internal/api/router.go`
- Create: `web/src/pages/EditorialList.tsx`
- Create: `web/src/pages/EditorialDetail.tsx`
- Modify: `web/src/pages/ProblemDetail.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Create editorial handler**

```go
// internal/api/handler/editorial.go
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

type EditorialHandler struct {
	store *postgres.EditorialStore
}

func NewEditorialHandler(s *postgres.EditorialStore) *EditorialHandler {
	return &EditorialHandler{store: s}
}

func (h *EditorialHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	var req model.CreateEditorialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	e := &model.Editorial{
		ProblemID:        req.ProblemID,
		UserID:           claims.UserID,
		Title:            req.Title,
		Content:          req.Content,
		SolutionCode:     req.SolutionCode,
		SolutionLanguage: req.SolutionLanguage,
		Approach:         req.Approach,
		TimeComplexity:   req.TimeComplexity,
		SpaceComplexity:  req.SpaceComplexity,
	}
	
	if req.ContestID != "" {
		e.ContestID = &req.ContestID
	}
	
	if err := h.store.Create(r.Context(), e); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusCreated, e)
}

func (h *EditorialHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if e == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, e)
}

func (h *EditorialHandler) GetByProblem(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "problemId")
	editorials, err := h.store.GetByProblem(r.Context(), problemID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": editorials})
}

func (h *EditorialHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	
	items, total, _ := h.store.List(r.Context(), offset, limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}
```

- [ ] **Step 2: Add routes**

```go
r.Route("/api/editorials", func(r chi.Router) {
	r.Get("/", editorialH.List)
	r.Get("/{id}", editorialH.GetByID)
	r.Get("/problem/{problemId}", editorialH.GetByProblem)
	
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/", editorialH.Create)
	})
})
```

- [ ] **Step 3: Add editorial API calls**

```typescript
editorials: {
    list: (offset = 0, limit = 20) =>
        request<{ data: any[]; total: number }>(`/editorials?offset=${offset}&limit=${limit}`),
    get: (id: string) => request<any>(`/editorials/${id}`),
    getByProblem: (problemId: string) => request<any>(`/editorials/problem/${problemId}`),
    create: (data: any) => request<any>('/editorials', { method: 'POST', body: JSON.stringify(data) }),
},
```

- [ ] **Step 4: Add editorial section to ProblemDetail**

```tsx
// Add to ProblemDetail
const [editorials, setEditorials] = useState<any[]>([]);

// Fetch editorials
useEffect(() => {
  if (problem?.id) {
    api.editorials.getByProblem(problem.id)
      .then(d => setEditorials(d.data || []))
      .catch(() => {});
  }
}, [problem?.id]);

// Add editorials tab
{activeTab === 'editorials' && (
  <div className="space-y-4">
    {editorials.length === 0 ? (
      <p className="text-gray-500">No editorials yet.</p>
    ) : (
      editorials.map(e => (
        <Link key={e.id} to={`/editorials/${e.id}`} className="block border rounded p-4 hover:bg-gray-50">
          <div className="flex items-center gap-2">
            {e.is_official && <span className="bg-green-100 text-green-800 text-xs px-2 py-0.5 rounded">Official</span>}
            <h4 className="font-medium">{e.title}</h4>
          </div>
          <div className="flex gap-4 mt-2 text-sm text-gray-500">
            <span>{e.username}</span>
            <span>{e.time_complexity}</span>
            <span>{e.upvotes} votes</span>
          </div>
        </Link>
      ))
    )}
  </div>
)}
```

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/editorial.go internal/api/router.go web/src/pages/Editorial*.tsx web/src/pages/ProblemDetail.tsx web/src/App.tsx web/src/lib/api.ts
git commit -m "feat(editorial): add editorial functionality"
```

---

## Verification Checklist

- [ ] Editorials can be created
- [ ] Editorials display on problem page
- [ ] Official editorials marked
- [ ] Solution code displays
- [ ] Time/space complexity shown

---

## Notes

1. **Official editorials**: Created by problem setter
2. **Community editorials**: Anyone can contribute
3. **Solution code**: Optional code solution
4. **Complexity**: Time and space complexity analysis
