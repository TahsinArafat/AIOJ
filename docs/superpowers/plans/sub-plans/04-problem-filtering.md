# Sub-Plan 04: Problem Filtering

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Advanced problem filtering by tags, difficulty, solved status, and rating range.

**Architecture:** Extend problem store with filtering capabilities, add frontend filter UI.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Modify
- `internal/store/postgres/problems.go` - Add filtering methods
- `internal/api/handler/problem.go` - Add filter parameters

### Frontend Files to Create
- `web/src/components/ProblemFilters.tsx` - Filter component
- `web/src/components/TagSelector.tsx` - Tag selection component

### Frontend Files to Modify
- `web/src/pages/ProblemList.tsx` - Add filter UI
- `web/src/lib/api.ts` - Add filter parameters

---

## Tasks

### Task 1: Backend Problem Filtering

**Files:**
- Modify: `internal/store/postgres/problems.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Update ProblemStore interface**

Add to `internal/store/interfaces.go`:

```go
type ProblemFilter struct {
	Difficulty   string
	Tags         []string
	MinRating    *int
	MaxRating    *int
	SolvedByUser *string // User ID to filter solved problems
	Search       string
}

type ProblemStore interface {
	// ... existing methods ...
	ListWithFilter(ctx context.Context, offset, limit int, filter ProblemFilter) ([]model.ProblemListItem, int, error)
	GetAllTags(ctx context.Context) ([]string, error)
}
```

- [ ] **Step 2: Implement filtered list**

Add to `internal/store/postgres/problems.go`:

```go
func (s *ProblemStore) ListWithFilter(ctx context.Context, offset, limit int, filter store.ProblemFilter) ([]model.ProblemListItem, int, error) {
	// Build query dynamically
	where := []string{"p.visible = true"}
	args := []interface{}{}
	argIdx := 1
	
	if filter.Difficulty != "" {
		where = append(where, fmt.Sprintf("p.difficulty = $%d", argIdx))
		args = append(args, filter.Difficulty)
		argIdx++
	}
	
	if len(filter.Tags) > 0 {
		where = append(where, fmt.Sprintf("p.tags && $%d", argIdx))
		args = append(args, pq.Array(filter.Tags))
		argIdx++
	}
	
	if filter.Search != "" {
		where = append(where, fmt.Sprintf("(p.title ILIKE $%d OR p.slug ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}
	
	// Count query
	countQuery := "SELECT COUNT(*) FROM problems p WHERE " + strings.Join(where, " AND ")
	var total int
	s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	
	// Main query
	selectQuery := `SELECT p.id, p.slug, p.title, p.difficulty, p.tags, p.submission_count, p.accepted_count, p.source
	                FROM problems p`
	
	if filter.SolvedByUser != nil {
		selectQuery += ` LEFT JOIN submissions s ON p.id = s.problem_id AND s.user_id = $` + strconv.Itoa(argIdx) + ` AND s.status = 'ac'`
		args = append(args, *filter.SolvedByUser)
		argIdx++
		where = append(where, "s.id IS NOT NULL") // Only solved problems
	}
	
	selectQuery += " WHERE " + strings.Join(where, " AND ")
	selectQuery += fmt.Sprintf(" ORDER BY p.created_at DESC OFFSET $%d LIMIT $%d", argIdx, argIdx+1)
	args = append(args, offset, limit)
	
	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var items []model.ProblemListItem
	for rows.Next() {
		var p model.ProblemListItem
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Difficulty, pq.Array(&p.Tags), &p.SubmissionCount, &p.AcceptedCount, &p.Source); err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if items == nil {
		items = []model.ProblemListItem{}
	}
	return items, total, nil
}

func (s *ProblemStore) GetAllTags(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT DISTINCT unnest(tags) FROM problems ORDER BY 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/problems.go
git commit -m "feat(filter): add problem filtering to store"
```

---

### Task 2: Update Problem Handler

**Files:**
- Modify: `internal/api/handler/problem.go`

- [ ] **Step 1: Add filter parameters to List handler**

```go
func (h *ProblemHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	
	filter := store.ProblemFilter{
		Difficulty: r.URL.Query().Get("difficulty"),
		Search:     r.URL.Query().Get("search"),
	}
	
	if tagsStr := r.URL.Query().Get("tags"); tagsStr != "" {
		filter.Tags = strings.Split(tagsStr, ",")
	}
	
	if solvedBy := r.URL.Query().Get("solved_by"); solvedBy != "" {
		filter.SolvedByUser = &solvedBy
	}
	
	items, total, _ := h.store.ListWithFilter(r.Context(), offset, limit, filter)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *ProblemHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, _ := h.store.GetAllTags(r.Context())
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": tags})
}
```

- [ ] **Step 2: Add route for tags**

Add to router:
```go
r.Get("/api/problems/tags", problemH.ListTags)
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/problem.go internal/api/router.go
git commit -m "feat(filter): add filter parameters to problem API"
```

---

### Task 3: Frontend Filter Components

**Files:**
- Create: `web/src/components/TagSelector.tsx`
- Create: `web/src/components/ProblemFilters.tsx`

- [ ] **Step 1: Create TagSelector component**

```tsx
// web/src/components/TagSelector.tsx
import { useState, useEffect } from 'react';
import { api } from '../lib/api';

interface TagSelectorProps {
  selectedTags: string[];
  onChange: (tags: string[]) => void;
}

export default function TagSelector({ selectedTags, onChange }: TagSelectorProps) {
  const [availableTags, setAvailableTags] = useState<string[]>([]);
  const [isOpen, setIsOpen] = useState(false);

  useEffect(() => {
    api.problems.listTags().then(d => setAvailableTags(d.data || [])).catch(() => {});
  }, []);

  const toggleTag = (tag: string) => {
    if (selectedTags.includes(tag)) {
      onChange(selectedTags.filter(t => t !== tag));
    } else {
      onChange([...selectedTags, tag]);
    }
  };

  return (
    <div className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="w-full px-3 py-2 text-left border rounded-lg bg-white"
      >
        {selectedTags.length > 0
          ? `${selectedTags.length} tags selected`
          : 'Select tags...'}
      </button>
      
      {isOpen && (
        <div className="absolute z-10 w-full mt-1 bg-white border rounded-lg shadow-lg max-h-60 overflow-y-auto">
          {availableTags.map(tag => (
            <label
              key={tag}
              className="flex items-center px-3 py-2 hover:bg-gray-50 cursor-pointer"
            >
              <input
                type="checkbox"
                checked={selectedTags.includes(tag)}
                onChange={() => toggleTag(tag)}
                className="mr-2"
              />
              {tag}
            </label>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Create ProblemFilters component**

```tsx
// web/src/components/ProblemFilters.tsx
import TagSelector from './TagSelector';

interface ProblemFiltersProps {
  difficulty: string;
  tags: string[];
  search: string;
  onDifficultyChange: (difficulty: string) => void;
  onTagsChange: (tags: string[]) => void;
  onSearchChange: (search: string) => void;
  onClear: () => void;
}

export default function ProblemFilters({
  difficulty,
  tags,
  search,
  onDifficultyChange,
  onTagsChange,
  onSearchChange,
  onClear,
}: ProblemFiltersProps) {
  const difficulties = ['', 'easy', 'medium', 'hard'];
  
  return (
    <div className="bg-white border rounded-lg p-4 space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">Filters</h3>
        <button onClick={onClear} className="text-sm text-blue-600 hover:text-blue-800">
          Clear all
        </button>
      </div>
      
      {/* Search */}
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Search</label>
        <input
          type="text"
          value={search}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder="Search problems..."
          className="w-full px-3 py-2 border rounded-lg"
        />
      </div>
      
      {/* Difficulty */}
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Difficulty</label>
        <div className="flex gap-2">
          {difficulties.map(d => (
            <button
              key={d}
              onClick={() => onDifficultyChange(d)}
              className={`px-3 py-1.5 rounded text-sm ${
                difficulty === d
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
              }`}
            >
              {d || 'All'}
            </button>
          ))}
        </div>
      </div>
      
      {/* Tags */}
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">Tags</label>
        <TagSelector selectedTags={tags} onChange={onTagsChange} />
      </div>
      
      {/* Selected Tags */}
      {tags.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {tags.map(tag => (
            <span
              key={tag}
              className="inline-flex items-center px-2 py-1 bg-blue-100 text-blue-800 rounded text-sm"
            >
              {tag}
              <button
                onClick={() => onTagsChange(tags.filter(t => t !== tag))}
                className="ml-1 hover:text-blue-600"
              >
                ×
              </button>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/components/TagSelector.tsx web/src/components/ProblemFilters.tsx
git commit -m "feat(filter): add filter UI components"
```

---

### Task 4: Update Problem List Page

**Files:**
- Modify: `web/src/pages/ProblemList.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Update API with filter support**

```typescript
problems: {
    list: (offset = 0, limit = 20, filters?: {
        difficulty?: string;
        tags?: string[];
        search?: string;
    }) => {
        let url = `/problems?offset=${offset}&limit=${limit}`;
        if (filters?.difficulty) url += `&difficulty=${filters.difficulty}`;
        if (filters?.tags?.length) url += `&tags=${filters.tags.join(',')}`;
        if (filters?.search) url += `&search=${encodeURIComponent(filters.search)}`;
        return request<{ data: any[]; total: number }>(url);
    },
    listTags: () => request<{ data: string[] }>('/problems/tags'),
    // ... existing methods ...
},
```

- [ ] **Step 2: Update ProblemList page**

```tsx
// web/src/pages/ProblemList.tsx
import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../lib/api';
import ProblemFilters from '../components/ProblemFilters';

export default function ProblemList() {
  const [problems, setProblems] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [difficulty, setDifficulty] = useState('');
  const [tags, setTags] = useState<string[]>([]);
  const [search, setSearch] = useState('');
  const limit = 20;

  useEffect(() => {
    api.problems.list(offset, limit, { difficulty, tags, search }).then(d => {
      setProblems(d.data || []);
      setTotal(d.total || 0);
    }).catch(console.error);
  }, [offset, difficulty, tags, search]);

  const clearFilters = () => {
    setDifficulty('');
    setTags([]);
    setSearch('');
    setOffset(0);
  };

  return (
    <div className="flex gap-6">
      {/* Sidebar Filters */}
      <aside className="w-64 flex-shrink-0">
        <ProblemFilters
          difficulty={difficulty}
          tags={tags}
          search={search}
          onDifficultyChange={(d) => { setDifficulty(d); setOffset(0); }}
          onTagsChange={(t) => { setTags(t); setOffset(0); }}
          onSearchChange={(s) => { setSearch(s); setOffset(0); }}
          onClear={clearFilters}
        />
      </aside>

      {/* Problem List */}
      <div className="flex-1">
        <h1 className="text-2xl font-bold mb-4">Problems</h1>
        
        <div className="border rounded-lg divide-y">
          {problems.map(p => (
            <Link key={p.id} to={`/problems/${p.slug}`} className="block px-4 py-3 hover:bg-gray-50">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="font-medium">{p.title}</h3>
                  <div className="flex gap-2 mt-1">
                    {p.tags?.slice(0, 3).map((tag: string) => (
                      <span key={tag} className="text-xs bg-gray-100 px-2 py-0.5 rounded">
                        {tag}
                      </span>
                    ))}
                  </div>
                </div>
                <div className="text-right">
                  <span className={`text-sm font-medium ${
                    p.difficulty === 'easy' ? 'text-green-600' :
                    p.difficulty === 'hard' ? 'text-red-600' : 'text-yellow-600'
                  }`}>
                    {p.difficulty}
                  </span>
                  <p className="text-xs text-gray-500 mt-1">
                    {p.accepted_count}/{p.submission_count} solved
                  </p>
                </div>
              </div>
            </Link>
          ))}
        </div>

        {/* Pagination */}
        <div className="flex justify-between mt-4">
          <span className="text-sm text-gray-500">{total} problems</span>
          <div className="flex gap-2">
            <button
              onClick={() => setOffset(Math.max(0, offset - limit))}
              disabled={offset === 0}
              className="px-3 py-1 border rounded disabled:opacity-40"
            >
              Prev
            </button>
            <button
              onClick={() => setOffset(offset + limit)}
              disabled={offset + limit >= total}
              className="px-3 py-1 border rounded disabled:opacity-40"
            >
              Next
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 3: Test the UI**

Run: `cd web && npm run dev`
Verify all filters work correctly

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/ProblemList.tsx web/src/lib/api.ts
git commit -m "feat(filter): add filter UI to problem list page"
```

---

## Verification Checklist

After completing all tasks:

- [ ] Tag list loads from database
- [ ] Difficulty filter works
- [ ] Tag filter works (multiple selection)
- [ ] Search filter works
- [ ] Filters can be combined
- [ ] Clear filters works
- [ ] Pagination resets on filter change
- [ ] Problem count updates correctly

---

## Notes

1. **Tags**: Loaded dynamically from database
2. **Multiple tags**: Can select multiple tags (OR logic)
3. **Search**: Searches title and slug
4. **Pagination**: Resets when filters change
5. **Performance**: Indexes on tags (GIN) and difficulty
