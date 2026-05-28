# Sub-Plan 03: Division System

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement contest divisions (Div 1/2/3/4) with rating-based eligibility to ensure fair competition.

**Architecture:** Extend contest model with division fields, add eligibility checks, frontend division display and filtering.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Modify
- `internal/model/contest.go` - Add division constants and validation
- `internal/store/postgres/contests.go` - Add division filtering
- `internal/api/handler/contest.go` - Add division validation

### Frontend Files to Create
- `web/src/lib/divisions.ts` - Division utilities
- `web/src/components/DivisionBadge.tsx` - Division badge component

### Frontend Files to Modify
- `web/src/pages/ContestList.tsx` - Add division filter
- `web/src/pages/ContestDetail.tsx` - Show division info
- `web/src/pages/ContestCreate.tsx` - Add division selector

---

## Tasks

### Task 1: Division Constants and Models

**Files:**
- Modify: `internal/model/contest.go`

- [ ] **Step 1: Add division constants**

Add to `internal/model/contest.go`:

```go
// Division constants
const (
	DivisionNone = 0 // Unrated or open
	Division1    = 1 // Rating >= 1900
	Division2    = 2 // Rating < 2100
	Division3    = 3 // Rating < 1600
	Division4    = 4 // Rating < 1400
)

// DivisionNames maps division number to display name
var DivisionNames = map[int]string{
	DivisionNone: "Open",
	Division1:    "Div. 1",
	Division2:    "Div. 2",
	Division3:    "Div. 3",
	Division4:    "Div. 4",
}

// DivisionColors maps division to color
var DivisionColors = map[int]string{
	DivisionNone: "#808080",
	Division1:    "#FF0000",
	Division2:    "#0000FF",
	Division3:    "#008000",
	Division4:    "#808080",
}

// GetDivisionRange returns the rating range for a division
func GetDivisionRange(division int) (min, max int) {
	switch division {
	case Division1:
		return 1900, 9999
	case Division2:
		return 0, 2099
	case Division3:
		return 0, 1599
	case Division4:
		return 0, 1399
	default:
		return 0, 9999
	}
}

// IsEligibleForDivision checks if a user rating is eligible for a division
func IsEligibleForDivision(division, rating int) bool {
	min, max := GetDivisionRange(division)
	return rating >= min && rating <= max
}

// GetEligibleDivisions returns all divisions a user can participate in
func GetEligibleDivisions(rating int) []int {
	var eligible []int
	for div := Division1; div <= Division4; div++ {
		if IsEligibleForDivision(div, rating) {
			eligible = append(eligible, div)
		}
	}
	if len(eligible) == 0 {
		eligible = []int{DivisionNone}
	}
	return eligible
}
```

- [ ] **Step 2: Add division validation to CreateContestRequest**

Update `CreateContestRequest` struct:

```go
type CreateContestRequest struct {
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Division    int        `json:"division"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     time.Time  `json:"end_time"`
	FreezeTime  *time.Time `json:"freeze_time,omitempty"`
	Password    string     `json:"password,omitempty"`
	Description string     `json:"description,omitempty"`
	ProblemIDs  []string   `json:"problem_ids"`
}
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/model/...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add internal/model/contest.go
git commit -m "feat(division): add division constants and eligibility checks"
```

---

### Task 2: Division Filtering in Store

**Files:**
- Modify: `internal/store/postgres/contests.go`

- [ ] **Step 1: Add division filter to List method**

Update the `List` method to support division filtering:

```go
func (s *ContestStore) List(ctx context.Context, offset, limit int, division *int) ([]model.Contest, int, error) {
	var total int
	query := "SELECT COUNT(*) FROM contests WHERE visible=true"
	args := []interface{}{}
	
	if division != nil {
		query += " AND division = $1"
		args = append(args, *division)
	}
	s.db.QueryRowContext(ctx, query, args...).Scan(&total)
	
	rowsQuery := `SELECT id,title,type,division,start_time,end_time,visible,description,created_at
		          FROM contests WHERE visible=true`
	if division != nil {
		rowsQuery += " AND division = $1"
	}
	rowsQuery += " ORDER BY start_time DESC OFFSET $2 LIMIT $3"
	args = append(args, offset, limit)
	
	rows, err := s.db.QueryContext(ctx, rowsQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var items []model.Contest
	for rows.Next() {
		var c model.Contest
		rows.Scan(&c.ID, &c.Title, &c.Type, &c.Division, &c.StartTime, &c.EndTime, &c.Visible, &c.Description, &c.CreatedAt)
		items = append(items, c)
	}
	if items == nil {
		items = []model.Contest{}
	}
	return items, total, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/store/postgres/contests.go
git commit -m "feat(division): add division filtering to contest list"
```

---

### Task 3: Update Contest Handler

**Files:**
- Modify: `internal/api/handler/contest.go`

- [ ] **Step 1: Add division to contest creation**

Update the `Create` handler to include division:

```go
func (h *ContestHandler) Create(w http.ResponseWriter, r *http.Request) {
	// ... existing validation ...
	
	if req.Division < model.DivisionNone || req.Division > model.Division4 {
		http.Error(w, "invalid division", http.StatusBadRequest)
		return
	}
	
	c := &model.Contest{
		// ... existing fields ...
		Division: req.Division,
	}
	
	// ... rest of creation ...
}
```

- [ ] **Step 2: Add division filter to List handler**

Update the `List` handler:

```go
func (h *ContestHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	
	var division *int
	if divStr := r.URL.Query().Get("division"); divStr != "" {
		d, err := strconv.Atoi(divStr)
		if err == nil {
			division = &d
		}
	}
	
	items, total, _ := h.store.List(r.Context(), offset, limit, division)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}
```

- [ ] **Step 3: Add eligibility check**

Add a helper to check if user is eligible for contest:

```go
func (h *ContestHandler) CheckEligibility(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"eligible": false,
			"reason":   "not logged in",
		})
		return
	}
	
	contestID := chi.URLParam(r, "id")
	contest, err := h.store.GetByID(r.Context(), contestID)
	if err != nil || contest == nil {
		http.Error(w, "contest not found", http.StatusNotFound)
		return
	}
	
	// Get user rating
	profile, _ := h.userStore.GetProfile(r.Context(), claims.UserID)
	rating := 0
	if profile != nil {
		rating = profile.Rating
	}
	
	eligible := contest.Division == model.DivisionNone || 
		model.IsEligibleForDivision(contest.Division, rating)
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"eligible":    eligible,
		"division":    contest.Division,
		"user_rating": rating,
	})
}
```

- [ ] **Step 4: Add route**

Add to router:
```go
r.Get("/api/contests/{id}/eligibility", contestH.CheckEligibility)
```

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/contest.go internal/api/router.go
git commit -m "feat(division): add division validation and eligibility check"
```

---

### Task 4: Frontend Division Utilities

**Files:**
- Create: `web/src/lib/divisions.ts`
- Create: `web/src/components/DivisionBadge.tsx`

- [ ] **Step 1: Create division utilities**

```typescript
// web/src/lib/divisions.ts

export const DIVISIONS = {
  0: { name: 'Open', color: '#808080', min: 0, max: 9999 },
  1: { name: 'Div. 1', color: '#FF0000', min: 1900, max: 9999 },
  2: { name: 'Div. 2', color: '#0000FF', min: 0, max: 2099 },
  3: { name: 'Div. 3', color: '#008000', min: 0, max: 1599 },
  4: { name: 'Div. 4', color: '#808080', min: 0, max: 1399 },
} as const;

export type Division = keyof typeof DIVISIONS;

export function getDivisionInfo(division: Division) {
  return DIVISIONS[division] || DIVISIONS[0];
}

export function isEligibleForDivision(division: Division, rating: number): boolean {
  const info = getDivisionInfo(division);
  return rating >= info.min && rating <= info.max;
}

export function getEligibleDivisions(rating: number): Division[] {
  return (Object.keys(DIVISIONS) as unknown as Division[]).filter(
    (d) => d === 0 || isEligibleForDivision(d, rating)
  );
}

export function getDivisionName(division: Division): string {
  return getDivisionInfo(division).name;
}

export function getDivisionColor(division: Division): string {
  return getDivisionInfo(division).color;
}
```

- [ ] **Step 2: Create DivisionBadge component**

```tsx
// web/src/components/DivisionBadge.tsx
import { getDivisionName, getDivisionColor, type Division } from '../lib/divisions';

interface DivisionBadgeProps {
  division: Division;
  size?: 'sm' | 'md' | 'lg';
}

export default function DivisionBadge({ division, size = 'md' }: DivisionBadgeProps) {
  const name = getDivisionName(division);
  const color = getDivisionColor(division);
  
  const sizeClasses = {
    sm: 'text-xs px-1.5 py-0.5',
    md: 'text-sm px-2 py-1',
    lg: 'text-base px-3 py-1.5',
  };
  
  if (division === 0) {
    return (
      <span className={`inline-flex items-center font-medium rounded bg-gray-100 text-gray-600 ${sizeClasses[size]}`}>
        {name}
      </span>
    );
  }
  
  return (
    <span
      className={`inline-flex items-center font-medium rounded ${sizeClasses[size]}`}
      style={{ color, backgroundColor: `${color}20` }}
    >
      {name}
    </span>
  );
}
```

- [ ] **Step 3: Verify TypeScript compilation**

Run: `cd web && npm run build`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/divisions.ts web/src/components/DivisionBadge.tsx
git commit -m "feat(division): add frontend division utilities and badge"
```

---

### Task 5: Update Contest List with Division Filter

**Files:**
- Modify: `web/src/pages/ContestList.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Update API to support division filter**

Update `web/src/lib/api.ts`:

```typescript
contests: {
    list: (offset = 0, limit = 20, division?: number) => {
        let url = `/contests?offset=${offset}&limit=${limit}`;
        if (division !== undefined) {
            url += `&division=${division}`;
        }
        return request<{ data: any[]; total: number }>(url);
    },
    // ... existing methods ...
},
```

- [ ] **Step 2: Add division filter to ContestList**

```tsx
// web/src/pages/ContestList.tsx
import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../lib/api';
import DivisionBadge from '../components/DivisionBadge';
import { DIVISIONS, type Division } from '../lib/divisions';

export default function ContestList() {
  const [contests, setContests] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [division, setDivision] = useState<Division | undefined>(undefined);
  const limit = 20;

  useEffect(() => {
    api.contests.list(offset, limit, division).then(d => {
      setContests(d.data || []);
      setTotal(d.total || 0);
    }).catch(console.error);
  }, [offset, division]);

  return (
    <div>
      <h1 className="text-2xl font-bold mb-4">Contests</h1>
      
      {/* Division Filter */}
      <div className="flex gap-2 mb-6 flex-wrap">
        <button
          onClick={() => setDivision(undefined)}
          className={`px-3 py-1.5 rounded text-sm ${
            division === undefined
              ? 'bg-blue-600 text-white'
              : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
          }`}
        >
          All
        </button>
        {(Object.entries(DIVISIONS) as [string, typeof DIVISIONS[0]][]).map(([key, info]) => (
          <button
            key={key}
            onClick={() => setDivision(Number(key) as Division)}
            className={`px-3 py-1.5 rounded text-sm ${
              division === Number(key)
                ? 'text-white'
                : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
            }`}
            style={division === Number(key) ? { backgroundColor: info.color } : {}}
          >
            {info.name}
          </button>
        ))}
      </div>

      {/* Contest List */}
      <div className="space-y-4">
        {contests.map(c => (
          <Link key={c.id} to={`/contests/${c.id}`} className="block">
            <div className="border rounded-lg p-4 hover:bg-gray-50">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="font-semibold">{c.title}</h3>
                  <p className="text-sm text-gray-600 mt-1">{c.description}</p>
                </div>
                <DivisionBadge division={c.division} />
              </div>
              <div className="flex gap-4 mt-2 text-sm text-gray-500">
                <span>{new Date(c.start_time).toLocaleString()}</span>
                <span className="uppercase">{c.type}</span>
              </div>
            </div>
          </Link>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Test the UI**

Run: `cd web && npm run dev`
Verify division filter works correctly

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/ContestList.tsx web/src/lib/api.ts
git commit -m "feat(division): add division filter to contest list"
```

---

### Task 6: Show Division in Contest Detail

**Files:**
- Modify: `web/src/pages/ContestDetail.tsx`

- [ ] **Step 1: Add division display to ContestDetail**

```tsx
// Add import
import DivisionBadge from '../components/DivisionBadge';

// In the contest detail, add division badge next to title
<div className="flex items-start justify-between">
  <div>
    <div className="flex items-center gap-3">
      <h1 className="text-2xl font-bold">{contest.title}</h1>
      <DivisionBadge division={contest.division} />
    </div>
    {/* ... existing description and time info ... */}
  </div>
  {/* ... existing status badge ... */}
</div>
```

- [ ] **Step 2: Add eligibility warning**

```tsx
// Add state for eligibility
const [eligibility, setEligibility] = useState<any>(null);

// Check eligibility on load
useEffect(() => {
  if (id && getAccessToken()) {
    api.contests.checkEligibility(id).then(setEligibility).catch(() => {});
  }
}, [id]);

// Add eligibility warning
{eligibility && !eligibility.eligible && (
  <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
    <p className="text-yellow-800">
      ⚠️ Your rating ({eligibility.user_rating}) is not eligible for {getDivisionName(eligibility.division)}.
    </p>
  </div>
)}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ContestDetail.tsx
git commit -m "feat(division): show division and eligibility in contest detail"
```

---

## Verification Checklist

After completing all tasks:

- [ ] Division constants defined correctly
- [ ] Eligibility checks work for all divisions
- [ ] Contest list filters by division
- [ ] Division badge displays with correct color
- [ ] Eligibility warning shows for ineligible users
- [ ] Contest creation includes division field
- [ ] Division filter persists in URL (optional)

---

## Notes

1. **Division 0 (Open)**: Unrated contests, anyone can participate
2. **Division 1**: Rated for users with rating >= 1900
3. **Division 2**: Rated for users with rating < 2100
4. **Division 3**: Rated for users with rating < 1600
5. **Division 4**: Rated for users with rating < 1400
6. **Multiple divisions**: Users can participate in higher divisions out of competition
