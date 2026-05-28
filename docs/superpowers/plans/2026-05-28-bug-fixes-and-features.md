# AIOJ Bug Fixes & Feature Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all critical bugs and implement missing features to make AIOJ a production-ready Codeforces alternative.

**Architecture:** Fix broken links/routes first, then add template code, then improve auth handling, then add interactive problem support and IUPC hosting.

**Tech Stack:** Go + Chi (backend), React + TypeScript + Tailwind (frontend), PostgreSQL, Docker

---

## Phase 1: Critical Bug Fixes (Must Fix Immediately)

### Task 1: Fix Broken User Profile Links

**Problem:** Rankings.tsx and GlobalSearch.tsx link to `/users/` (plural) but the route is `/user/:username` (singular).

**Files:**
- Modify: `web/src/pages/Rankings.tsx:71`
- Modify: `web/src/components/GlobalSearch.tsx:79`

- [ ] **Step 1: Fix Rankings.tsx link**

In `web/src/pages/Rankings.tsx`, find line 71:
```tsx
<Link to={`/users/${u.username}`}
```
Change to:
```tsx
<Link to={`/user/${u.username}`}
```

- [ ] **Step 2: Fix GlobalSearch.tsx link**

In `web/src/components/GlobalSearch.tsx`, find line 79:
```tsx
to={`/users/${result.id}`}
```
Change to:
```tsx
to={`/user/${result.id}`}
```

Wait — GlobalSearch uses `result.id` but the route expects `username`. Need to check what the search API returns for users. Let me verify.

Actually, looking at the search endpoint response format, it returns `{ id, username, email, role, is_bot, created_at, updated_at }`. So the link should use `username`, not `id`:

```tsx
to={`/user/${result.username}`}
```

- [ ] **Step 3: Verify frontend builds**

Run: `npm run build --prefix web`
Expected: Clean build with zero TypeScript errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/Rankings.tsx web/src/components/GlobalSearch.tsx
git commit -m "fix: correct user profile links from /users/ to /user/"
```

---

### Task 2: Fix Problem Links Using UUID Instead of Slug

**Problem:** Several pages link to `/problems/${problem_id}` where `problem_id` is a UUID, but the route `/problems/:slug` expects a slug string.

**Files:**
- Modify: `web/src/pages/SubmissionDetail.tsx`
- Modify: `web/src/pages/ContestDetail.tsx`
- Modify: `web/src/pages/VirtualContest.tsx`

- [ ] **Step 1: Check SubmissionDetail.tsx**

Read `web/src/pages/SubmissionDetail.tsx` and find where it links to problems. Look for `/problems/${sub.problem_id}` or similar patterns. The submission object may have `problem_slug` or we may need to fetch it.

Check what fields the submission API returns. If it doesn't return `problem_slug`, we need to either:
1. Add `problem_slug` to the submission response (backend change)
2. Use the problem ID and add a redirect route
3. Fetch the problem separately

For now, check if `sub.problem_slug` exists in the response.

- [ ] **Step 2: Check ContestDetail.tsx**

Read `web/src/pages/ContestDetail.tsx` and find problem links. The contest problems may have `slug` or `problem_slug` field.

- [ ] **Step 3: Check VirtualContest.tsx**

Read `web/src/pages/VirtualContest.tsx` and find problem links.

- [ ] **Step 4: Fix links based on available data**

If `slug` is available in the response, use it. If not, we need to add it to the API response.

- [ ] **Step 5: Verify frontend builds**

Run: `npm run build --prefix web`
Expected: Clean build.

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/SubmissionDetail.tsx web/src/pages/ContestDetail.tsx web/src/pages/VirtualContest.tsx
git commit -m "fix: use problem slug instead of UUID in problem links"
```

---

### Task 3: Fix Educational Contest CHECK Constraint

**Problem:** Migration 000001 has `CHECK (type IN ('acm','oi','ioi','practice'))` but the code uses `ContestTypeEducational = "educational"`. Creating educational contests fails at the database level.

**Files:**
- Create: `internal/store/migrations/000019_fix_contest_type_check.up.sql`
- Create: `internal/store/migrations/000019_fix_contest_type_check.down.sql`

- [ ] **Step 1: Create migration to fix CHECK constraint**

Create `internal/store/migrations/000019_fix_contest_type_check.up.sql`:
```sql
-- Fix contest type CHECK constraint to include 'educational'
ALTER TABLE contests DROP CONSTRAINT IF EXISTS contests_type_check;
ALTER TABLE contests ADD CONSTRAINT contests_type_check 
  CHECK (type IN ('acm','oi','ioi','practice','educational'));
```

Create `internal/store/migrations/000019_fix_contest_type_check.down.sql`:
```sql
-- Revert contest type CHECK constraint
ALTER TABLE contests DROP CONSTRAINT IF EXISTS contests_type_check;
ALTER TABLE contests ADD CONSTRAINT contests_type_check 
  CHECK (type IN ('acm','oi','ioi','practice'));
```

- [ ] **Step 2: Run migration**

Run: `make migrate-up`
Expected: Migration applied successfully.

- [ ] **Step 3: Verify educational contest creation**

Test creating an educational contest via API:
```bash
curl -X POST http://localhost/api/contests/educational \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"title":"Test Educational","type":"educational","start_time":"2026-06-01T10:00:00Z","end_time":"2026-06-01T15:00:00Z"}'
```
Expected: 201 Created (not 500 error).

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000019_fix_contest_type_check.*
git commit -m "fix: add 'educational' to contest type CHECK constraint"
```

---

### Task 4: Fix Submissions Endpoint Slug vs ID Mismatch

**Problem:** `GET /api/problems/{slug}/submissions` passes slug to `ListByProblem()` but the store queries by `WHERE problem_id=$1` (expects UUID).

**Files:**
- Modify: `internal/api/handler/submission.go`
- OR Modify: `internal/store/postgres/submissions.go`

- [ ] **Step 1: Check the store implementation**

Read `internal/store/postgres/submissions.go` and find `ListByProblem`. Check if it queries by `problem_id` or `slug`.

- [ ] **Step 2: Fix the handler or store**

Option A: Change handler to resolve slug to ID first:
```go
func (h *SubmissionHandler) ListByProblem(w http.ResponseWriter, r *http.Request) {
    slug := chi.URLParam(r, "slug")
    // Resolve slug to problem ID
    problem, err := h.probStore.GetBySlug(r.Context(), slug)
    if err != nil || problem == nil {
        http.Error(w, "problem not found", http.StatusNotFound)
        return
    }
    // ... use problem.ID for the query
}
```

Option B: Change store to query by slug:
```go
func (s *SubmissionStore) ListByProblem(ctx context.Context, slug string, offset, limit int) ([]*model.Submission, int, error) {
    // JOIN with problems table to filter by slug
    query := `SELECT s.* FROM submissions s JOIN problems p ON s.problem_id = p.id WHERE p.slug = $1 ...`
}
```

Option A is cleaner (keep store queries by ID).

- [ ] **Step 3: Verify with test**

Test: `curl http://localhost/api/problems/a-plus-b-2/submissions`
Expected: Returns submissions (or empty array), not 500 error.

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler/submission.go
git commit -m "fix: resolve slug to problem ID in ListByProblem handler"
```

---

## Phase 2: User-Reported Issues

### Task 5: Add Default Template Code Per Language

**Problem:** The code editor in ProblemDetail.tsx starts empty. Users should see a "Hello World" template for each language.

**Files:**
- Modify: `web/src/pages/ProblemDetail.tsx`

- [ ] **Step 1: Add template code constants**

Add at the top of `ProblemDetail.tsx` (after imports):

```typescript
const TEMPLATE_CODE: Record<string, string> = {
    'cpp-gpp-64': `#include <iostream>
using namespace std;

int main() {
    // Your code here
    
    return 0;
}`,
    'cpp-gpp-32': `#include <iostream>
using namespace std;

int main() {
    // Your code here
    
    return 0;
}`,
    'c-gcc-64': `#include <stdio.h>

int main() {
    // Your code here
    
    return 0;
}`,
    'c-gcc-32': `#include <stdio.h>

int main() {
    // Your code here
    
    return 0;
}`,
    'cpp-clang': `#include <iostream>
using namespace std;

int main() {
    // Your code here
    
    return 0;
}`,
    'python': `# Your code here
`,
    'java': `import java.util.Scanner;

public class Main {
    public static void main(String[] args) {
        Scanner sc = new Scanner(System.in);
        // Your code here
        
    }
}`,
    'rust': `fn main() {
    // Your code here
    
}`,
    'nodejs': `// Your code here
`,
    'csharp': `using System;

class Program {
    static void Main() {
        // Your code here
        
    }
}`,
}
```

- [ ] **Step 2: Update code initialization**

Find the `useEffect` that loads saved code (around line 90-96):

```typescript
useEffect(() => {
    if (slug) {
        const key = `aioj_draft_${problem?.id || slug}_${lang}`
        const saved = localStorage.getItem(key)
        setCode(saved || '')
    }
}, [slug, problem?.id, lang])
```

Change to:

```typescript
useEffect(() => {
    if (slug) {
        const key = `aioj_draft_${problem?.id || slug}_${lang}`
        const saved = localStorage.getItem(key)
        setCode(saved || TEMPLATE_CODE[lang] || '')
    }
}, [slug, problem?.id, lang])
```

- [ ] **Step 3: Verify frontend builds**

Run: `npm run build --prefix web`
Expected: Clean build.

- [ ] **Step 4: Test in browser**

1. Go to `http://localhost/problems/a-plus-b-2`
2. Select "C++ (G++ 64-bit)" — should show C++ template
3. Select "Python 3" — should show Python template
4. Select "Java" — should show Java template
5. Refresh page — template should persist if no code written

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/ProblemDetail.tsx
git commit -m "feat: add default template code per language in problem editor"
```

---

### Task 6: Fix Practice Page Auth Handling

**Problem:** Practice page calls `api.recommendations.get()` without checking auth. Shows generic error when not logged in.

**Files:**
- Modify: `web/src/pages/Practice.tsx`

- [ ] **Step 1: Add auth check**

Add import at top:
```typescript
import { getAccessToken } from '../lib/api'
```

Add auth check before the API call:

```typescript
useEffect(() => {
    if (!getAccessToken()) {
        setLoading(false)
        return
    }
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
```

- [ ] **Step 2: Add login prompt for unauthenticated users**

Update the return statement to show login prompt:

```typescript
if (!getAccessToken()) {
    return (
        <div className="max-w-4xl mx-auto text-center py-20">
            <h1 className="text-3xl font-extrabold tracking-tight text-gray-900 mb-4">Personalized Practice</h1>
            <p className="text-gray-600 mb-6">Log in to get smart problem recommendations tailored to your rating and weak areas.</p>
            <a href="/login" className="bg-blue-600 text-white px-6 py-3 rounded-lg font-medium hover:bg-blue-700 transition-colors">
                Log In to Practice
            </a>
        </div>
    )
}
```

- [ ] **Step 3: Verify frontend builds**

Run: `npm run build --prefix web`
Expected: Clean build.

- [ ] **Step 4: Test in browser**

1. Open incognito window (not logged in)
2. Go to `http://localhost/practice`
3. Should see "Log In to Practice" message
4. Log in and go to `/practice` again
5. Should see recommendations (or "No recommendations" if no data)

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/Practice.tsx
git commit -m "fix: add auth check and login prompt to practice page"
```

---

### Task 7: Allow Teachers/Setters to Create Contests

**Problem:** Only admins can create contests. Teachers (approved setters) are blocked.

**Files:**
- Modify: `internal/api/handler/contest.go`

- [ ] **Step 1: Update Create handler**

Find the `Create` method (around line 27-31):

```go
func (h *ContestHandler) Create(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserClaims(r)
    if claims == nil || claims.Role != "admin" {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
```

Change to:

```go
func (h *ContestHandler) Create(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserClaims(r)
    if claims == nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }
    if claims.Role != "admin" && claims.Role != "teacher" {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
```

- [ ] **Step 2: Update CreateEducational handler**

Find the `CreateEducational` method (around line 363-367) and make the same change.

- [ ] **Step 3: Verify with test**

1. Login as a teacher account (or promote a user to teacher via admin)
2. Try creating a contest via API
3. Expected: 201 Created (not 403 Forbidden)

- [ ] **Step 4: Commit**

```bash
git add internal/api/handler/contest.go
git commit -m "feat: allow teachers to create contests"
```

---

### Task 8: Add Contest Update and Delete Endpoints

**Problem:** Once created, contests cannot be modified or removed.

**Files:**
- Modify: `internal/store/interfaces.go`
- Modify: `internal/store/postgres/contests.go`
- Modify: `internal/api/handler/contest.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Add Update and Delete to ContestStore interface**

In `internal/store/interfaces.go`, add to the ContestStore interface:

```go
type ContestStore interface {
    Create(ctx context.Context, c *model.Contest) error
    GetByID(ctx context.Context, id string) (*model.Contest, error)
    List(ctx context.Context, offset, limit int) ([]*model.ContestListItem, int, error)
    Update(ctx context.Context, c *model.Contest) error  // NEW
    Delete(ctx context.Context, id string) error  // NEW
    // ... existing methods
}
```

- [ ] **Step 2: Implement Update and Delete in postgres store**

In `internal/store/postgres/contests.go`, add:

```go
func (s *ContestStore) Update(ctx context.Context, c *model.Contest) error {
    _, err := s.db.ExecContext(ctx,
        `UPDATE contests SET title=$1, type=$2, start_time=$3, end_time=$4, 
         freeze_time=$5, password=$6, description=$7, visible=$8, updated_at=NOW()
         WHERE id=$9`,
        c.Title, c.Type, c.StartTime, c.EndTime, c.FreezeTime, c.Password, c.Description, c.Visible, c.ID)
    return err
}

func (s *ContestStore) Delete(ctx context.Context, id string) error {
    _, err := s.db.ExecContext(ctx, `DELETE FROM contests WHERE id=$1`, id)
    return err
}
```

- [ ] **Step 3: Add Update and Delete handlers**

In `internal/api/handler/contest.go`, add:

```go
func (h *ContestHandler) Update(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserClaims(r)
    if claims == nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }
    id := chi.URLParam(r, "id")
    // Check if user has access to this contest
    // ... (check permissions)
    
    var req model.CreateContestRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    
    c := &model.Contest{
        ID:          id,
        Title:       req.Title,
        Type:        req.Type,
        StartTime:   req.StartTime,
        EndTime:     req.EndTime,
        FreezeTime:  req.FreezeTime,
        Password:    req.Password,
        Description: req.Description,
    }
    
    if err := h.store.Update(r.Context(), c); err != nil {
        http.Error(w, "update failed", http.StatusInternalServerError)
        return
    }
    respondJSON(w, http.StatusOK, c)
}

func (h *ContestHandler) Delete(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserClaims(r)
    if claims == nil || claims.Role != "admin" {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    id := chi.URLParam(r, "id")
    if err := h.store.Delete(r.Context(), id); err != nil {
        http.Error(w, "delete failed", http.StatusInternalServerError)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Add routes**

In `internal/api/router.go`, inside the contests auth group:

```go
r.Put("/{id}", contestH.Update)
r.Delete("/{id}", contestH.Delete)
```

- [ ] **Step 5: Verify with tests**

Test update:
```bash
curl -X PUT http://localhost/api/contests/<id> \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"title":"Updated Title","type":"acm","start_time":"...","end_time":"..."}'
```

Test delete:
```bash
curl -X DELETE http://localhost/api/contests/<id> \
  -H "Authorization: Bearer <token>"
```

- [ ] **Step 6: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/contests.go internal/api/handler/contest.go internal/api/router.go
git commit -m "feat: add contest update and delete endpoints"
```

---

## Phase 3: Problem Setter Improvements

### Task 9: Add Sample Case Editor to SetterWorkspace

**Problem:** The setter workspace has no UI to add/edit/remove sample cases. Setters must use the API directly.

**Files:**
- Modify: `web/src/pages/SetterProblemWorkspace.tsx`

- [ ] **Step 1: Add sample cases state**

Add to the state declarations:

```typescript
// Sample Cases State
const [sampleCases, setSampleCases] = useState<{input: string; output: string; explanation: string}[]>([])
```

- [ ] **Step 2: Load sample cases from problem**

In the `loadProblem` function, add:

```typescript
setSampleCases(data.sample_cases || [])
```

- [ ] **Step 3: Add sample cases UI**

Add a new tab or section in the workspace for editing sample cases:

```typescript
{/* Sample Cases Section */}
<div className="space-y-4">
    <div className="flex items-center justify-between">
        <h3 className="font-semibold text-sm text-gray-700">Sample Cases</h3>
        <button
            onClick={() => setSampleCases([...sampleCases, {input: '', output: '', explanation: ''}])}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-xs hover:bg-blue-700"
        >
            + Add Sample
        </button>
    </div>
    
    {sampleCases.map((sc, i) => (
        <div key={i} className="border border-gray-200 rounded-lg p-4 space-y-3">
            <div className="flex items-center justify-between">
                <span className="font-medium text-sm">Sample {i + 1}</span>
                <button
                    onClick={() => setSampleCases(sampleCases.filter((_, j) => j !== i))}
                    className="text-red-600 hover:text-red-800 text-xs"
                >
                    Remove
                </button>
            </div>
            <div className="grid grid-cols-2 gap-3">
                <div>
                    <label className="block text-xs font-medium text-gray-500 mb-1">Input</label>
                    <textarea
                        value={sc.input}
                        onChange={e => {
                            const newCases = [...sampleCases]
                            newCases[i].input = e.target.value
                            setSampleCases(newCases)
                        }}
                        rows={3}
                        className="w-full font-mono text-xs border border-gray-300 rounded p-2"
                    />
                </div>
                <div>
                    <label className="block text-xs font-medium text-gray-500 mb-1">Expected Output</label>
                    <textarea
                        value={sc.output}
                        onChange={e => {
                            const newCases = [...sampleCases]
                            newCases[i].output = e.target.value
                            setSampleCases(newCases)
                        }}
                        rows={3}
                        className="w-full font-mono text-xs border border-gray-300 rounded p-2"
                    />
                </div>
            </div>
            <div>
                <label className="block text-xs font-medium text-gray-500 mb-1">Explanation (optional)</label>
                <input
                    type="text"
                    value={sc.explanation}
                    onChange={e => {
                        const newCases = [...sampleCases]
                        newCases[i].explanation = e.target.value
                        setSampleCases(newCases)
                    }}
                    className="w-full text-sm border border-gray-300 rounded px-2 py-1.5"
                />
            </div>
        </div>
    ))}
</div>
```

- [ ] **Step 4: Save sample cases with problem update**

In the save function, include sample cases in the update payload:

```typescript
const saveProblem = async () => {
    // ... existing code
    await api.problems.update(slug, {
        // ... existing fields
        sample_cases: sampleCases,
    })
}
```

- [ ] **Step 5: Verify frontend builds**

Run: `npm run build --prefix web`
Expected: Clean build.

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/SetterProblemWorkspace.tsx
git commit -m "feat: add sample case editor to setter workspace"
```

---

### Task 10: Add Interactive Problem Support

**Problem:** No support for interactive problems (where judge and solution communicate via stdin/stdout).

**Files:**
- Create: `internal/store/migrations/000020_interactive_problems.up.sql`
- Create: `internal/store/migrations/000020_interactive_problems.down.sql`
- Modify: `internal/model/problem.go`
- Modify: `internal/judge/worker.go`
- Modify: `web/src/pages/SetterProblemWorkspace.tsx`

- [ ] **Step 1: Create migration for interactive fields**

Create `internal/store/migrations/000020_interactive_problems.up.sql`:
```sql
-- Add interactive problem support
ALTER TABLE problems ADD COLUMN IF NOT EXISTS interactive BOOLEAN DEFAULT FALSE;
ALTER TABLE problems ADD COLUMN IF NOT EXISTS interactor_language VARCHAR(32);
ALTER TABLE problems ADD COLUMN IF NOT EXISTS interactor_source_code TEXT;
```

Create `internal/store/migrations/000020_interactive_problems.down.sql`:
```sql
-- Remove interactive problem support
ALTER TABLE problems DROP COLUMN IF EXISTS interactive;
ALTER TABLE problems DROP COLUMN IF EXISTS interactor_language;
ALTER TABLE problems DROP COLUMN IF EXISTS interactor_source_code;
```

- [ ] **Step 2: Update Problem model**

In `internal/model/problem.go`, add fields:

```go
type Problem struct {
    // ... existing fields
    Interactive           bool   `json:"interactive"`
    InteractorLanguage    string `json:"interactor_language,omitempty"`
    InteractorSourceCode  string `json:"interactor_source_code,omitempty"`
}
```

- [ ] **Step 3: Update judge worker for interactive mode**

In `internal/judge/worker.go`, add interactive judging logic:

```go
if problem.Interactive {
    // 1. Compile interactor
    // 2. Run solution and interactor in parallel
    // 3. Pipe stdout of solution to stdin of interactor
    // 4. Pipe stdout of interactor to stdin of solution
    // 5. Check interactor exit code for verdict
}
```

This is complex and may require using go-judge's process communication features.

- [ ] **Step 4: Update setter workspace UI**

Add interactive problem configuration in the setter workspace:

```tsx
{/* Interactive Problem Toggle */}
<div className="flex items-center gap-2">
    <input
        type="checkbox"
        id="interactive"
        checked={interactive}
        onChange={e => setInteractive(e.target.checked)}
        className="rounded"
    />
    <label htmlFor="interactive" className="text-sm font-medium text-gray-700">
        Interactive Problem
    </label>
</div>

{interactive && (
    <div className="space-y-3 pl-6 border-l-2 border-blue-200">
        <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">Interactor Language</label>
            <select
                value={interactorLanguage}
                onChange={e => setInteractorLanguage(e.target.value)}
                className="border border-gray-300 rounded px-2 py-1.5 text-sm"
            >
                <option value="cpp-gpp-64">C++ (G++ 64-bit)</option>
                <option value="python">Python 3</option>
            </select>
        </div>
        <div>
            <label className="block text-xs font-medium text-gray-500 mb-1">Interactor Source Code</label>
            <CodeEditor
                language={interactorLanguage}
                value={interactorSourceCode}
                onChange={setInteractorSourceCode}
                height="200px"
            />
        </div>
    </div>
)}
```

- [ ] **Step 5: Run migration and verify**

Run: `make migrate-up`
Expected: Migration applied successfully.

- [ ] **Step 6: Commit**

```bash
git add internal/store/migrations/000020_interactive_problems.* internal/model/problem.go internal/judge/worker.go web/src/pages/SetterProblemWorkspace.tsx
git commit -m "feat: add interactive problem support"
```

---

## Phase 4: IUPC Hosting System

### Task 11: Add Team Contest Support

**Problem:** No support for team-based contests (IUPC format: 3 members per team, 5 hours).

**Files:**
- Create: `internal/store/migrations/000021_team_contests.up.sql`
- Create: `internal/store/migrations/000021_team_contests.down.sql`
- Modify: `internal/model/contest.go`
- Modify: `internal/api/handler/contest.go`
- Create: `web/src/pages/IUPCContestDetail.tsx`

- [ ] **Step 1: Create migration for team contest fields**

Create `internal/store/migrations/000021_team_contests.up.sql`:
```sql
-- Add team contest support
ALTER TABLE contests ADD COLUMN IF NOT EXISTS team_size INTEGER DEFAULT 1;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS is_team_contest BOOLEAN DEFAULT FALSE;

-- Team registrations table
CREATE TABLE IF NOT EXISTS team_registrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    registered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(contest_id, team_id)
);
```

Create `internal/store/migrations/000021_team_contests.down.sql`:
```sql
-- Remove team contest support
DROP TABLE IF EXISTS team_registrations;
ALTER TABLE contests DROP COLUMN IF EXISTS team_size;
ALTER TABLE contests DROP COLUMN IF EXISTS is_team_contest;
```

- [ ] **Step 2: Update Contest model**

In `internal/model/contest.go`, add fields:

```go
type Contest struct {
    // ... existing fields
    TeamSize      int  `json:"team_size"`
    IsTeamContest bool `json:"is_team_contest"`
}
```

- [ ] **Step 3: Add team registration handler**

Create handler for teams to register for contests:

```go
func (h *ContestHandler) RegisterTeam(w http.ResponseWriter, r *http.Request) {
    claims := middleware.GetUserClaims(r)
    if claims == nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }
    
    contestID := chi.URLParam(r, "id")
    var req struct {
        TeamID string `json:"team_id"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    
    // Verify user is team captain
    // Register team for contest
}
```

- [ ] **Step 4: Create IUPC contest detail page**

Create `web/src/pages/IUPCContestDetail.tsx` with:
- Team registration section
- Team scoreboard (aggregated scores)
- Clarification system
- Real-time monitoring for admins

- [ ] **Step 5: Run migration and verify**

Run: `make migrate-up`
Expected: Migration applied successfully.

- [ ] **Step 6: Commit**

```bash
git add internal/store/migrations/000021_team_contests.* internal/model/contest.go internal/api/handler/contest.go web/src/pages/IUPCContestDetail.tsx
git commit -m "feat: add team contest support for IUPC hosting"
```

---

### Task 12: Add Homepage Stats from API

**Problem:** Homepage stats are hardcoded (`{ problems: 100, users: 500, submissions: 10000 }`).

**Files:**
- Modify: `internal/api/handler/stats.go`
- Modify: `internal/api/router.go`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Add platform stats endpoint**

In `internal/api/handler/stats.go`, add:

```go
func (h *StatsHandler) GetPlatformStats(w http.ResponseWriter, r *http.Request) {
    // Query counts from database
    var stats struct {
        Problems    int `json:"problems"`
        Users       int `json:"users"`
        Submissions int `json:"submissions"`
    }
    
    h.db.QueryRow("SELECT COUNT(*) FROM problems WHERE visible=true").Scan(&stats.Problems)
    h.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.Users)
    h.db.QueryRow("SELECT COUNT(*) FROM submissions").Scan(&stats.Submissions)
    
    respondJSON(w, http.StatusOK, stats)
}
```

- [ ] **Step 2: Add route**

In `internal/api/router.go`, add:

```go
r.Get("/stats/platform", statsH.GetPlatformStats)
```

- [ ] **Step 3: Update frontend to fetch stats**

In `web/src/App.tsx`, update the Home component:

```typescript
const [stats, setStats] = useState({ problems: 0, users: 0, submissions: 0 })

useEffect(() => {
    Promise.all([
        api.contests.list(0, 5),
        api.blog.list(0, 3),
        fetch('/api/stats/platform').then(r => r.json()),
    ]).then(([contestData, blogData, statsData]) => {
        setContests(contestData.data || [])
        setPosts(blogData.data || [])
        setStats(statsData)
    }).catch(() => {}).finally(() => setLoading(false))
}, [])
```

- [ ] **Step 4: Verify frontend builds**

Run: `npm run build --prefix web`
Expected: Clean build.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handler/stats.go internal/api/router.go web/src/App.tsx
git commit -m "feat: fetch homepage stats from API instead of hardcoded values"
```

---

## Phase 5: Additional Improvements

### Task 13: Add Syntax Highlighting for All Languages

**Problem:** CodeEditor only has extensions for cpp, python, java, rust. Other languages fall back to C++ highlighting.

**Files:**
- Modify: `web/src/components/CodeEditor.tsx`

- [ ] **Step 1: Add language extensions**

```typescript
import { javascript } from '@codemirror/lang-javascript'
import { csharp } from '@replit/codemirror-lang-csharp'  // if available
```

Update the language map:

```typescript
const languageExtensions: Record<string, any> = {
    'cpp-gpp-64': cpp(),
    'cpp-gpp-32': cpp(),
    'c-gcc-64': cpp(),
    'c-gcc-32': cpp(),
    'cpp-clang': cpp(),
    'python': python(),
    'java': java(),
    'rust': rust(),
    'nodejs': javascript(),
    'csharp': cpp(),  // fallback until csharp extension added
}
```

- [ ] **Step 2: Verify frontend builds**

Run: `npm run build --prefix web`
Expected: Clean build.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/CodeEditor.tsx
git commit -m "feat: add syntax highlighting for nodejs and csharp languages"
```

---

### Task 14: Add Notifications List Page

**Problem:** Only NotificationPreferences exists. No page to view past notifications.

**Files:**
- Create: `web/src/pages/Notifications.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/components/Navbar.tsx`

- [ ] **Step 1: Create Notifications page**

Create `web/src/pages/Notifications.tsx`:

```tsx
import { useEffect, useState } from 'react'
import { api } from '../lib/api'

export default function Notifications() {
    const [notifications, setNotifications] = useState<any[]>([])
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.notifications.list(false, 100)
            .then(d => setNotifications(d.data || []))
            .catch(console.error)
            .finally(() => setLoading(false))
    }, [])

    const handleMarkAsRead = async (id: string) => {
        await api.notifications.markAsRead(id)
        setNotifications(prev => prev.map(n => n.id === id ? { ...n, read: true } : n))
    }

    if (loading) return <div className="text-center py-20 text-gray-400">Loading...</div>

    return (
        <div className="max-w-2xl mx-auto">
            <h1 className="text-2xl font-bold mb-6">Notifications</h1>
            {notifications.length === 0 ? (
                <p className="text-gray-400 text-center py-8">No notifications yet.</p>
            ) : (
                <div className="space-y-2">
                    {notifications.map(n => (
                        <div
                            key={n.id}
                            className={`border rounded-lg p-4 ${n.read ? 'bg-white' : 'bg-blue-50'}`}
                        >
                            <div className="flex justify-between items-start">
                                <h3 className="font-medium">{n.title}</h3>
                                {!n.read && (
                                    <button
                                        onClick={() => handleMarkAsRead(n.id)}
                                        className="text-xs text-blue-600 hover:underline"
                                    >
                                        Mark as read
                                    </button>
                                )}
                            </div>
                            <p className="text-sm text-gray-600 mt-1">{n.content}</p>
                            <span className="text-xs text-gray-400 mt-2 block">
                                {new Date(n.created_at).toLocaleString()}
                            </span>
                        </div>
                    ))}
                </div>
            )}
        </div>
    )
}
```

- [ ] **Step 2: Add route and nav link**

In `web/src/App.tsx`, add import and route:
```tsx
import Notifications from './pages/Notifications'
// ...
<Route path="/notifications" element={<Notifications />} />
```

In `web/src/components/Navbar.tsx`, add link in the logged-in section:
```tsx
<Link to="/notifications" className="text-sm text-gray-600 hover:text-black">Notifications</Link>
```

- [ ] **Step 3: Verify frontend builds**

Run: `npm run build --prefix web`
Expected: Clean build.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/Notifications.tsx web/src/App.tsx web/src/components/Navbar.tsx
git commit -m "feat: add notifications list page"
```

---

### Task 15: Add Password Reset

**Problem:** No way to reset forgotten passwords.

**Files:**
- Create: `internal/store/migrations/000022_password_reset.up.sql`
- Create: `internal/store/migrations/000022_password_reset.down.sql`
- Modify: `internal/api/handler/auth.go`
- Create: `web/src/pages/ForgotPassword.tsx`
- Create: `web/src/pages/ResetPassword.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create migration for password reset tokens**

Create `internal/store/migrations/000022_password_reset.up.sql`:
```sql
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

- [ ] **Step 2: Add forgot password handler**

In `internal/api/handler/auth.go`, add:

```go
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Email string `json:"email"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    
    // Generate token, store hash, send email
    // Always return success to prevent email enumeration
    respondJSON(w, http.StatusOK, map[string]string{"message": "If the email exists, a reset link has been sent"})
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Token    string `json:"token"`
        Password string `json:"password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }
    
    // Verify token, update password, mark token as used
}
```

- [ ] **Step 3: Create frontend pages**

Create `web/src/pages/ForgotPassword.tsx` and `web/src/pages/ResetPassword.tsx`.

- [ ] **Step 4: Add routes**

```tsx
<Route path="/forgot-password" element={<ForgotPassword />} />
<Route path="/reset-password" element={<ResetPassword />} />
```

- [ ] **Step 5: Commit**

```bash
git add internal/store/migrations/000022_password_reset.* internal/api/handler/auth.go web/src/pages/ForgotPassword.tsx web/src/pages/ResetPassword.tsx web/src/App.tsx
git commit -m "feat: add password reset flow"
```

---

## Summary

| Phase | Tasks | Priority |
|-------|-------|----------|
| Phase 1: Critical Bugs | Tasks 1-4 | Must fix immediately |
| Phase 2: User-Reported | Tasks 5-8 | High |
| Phase 3: Setter Improvements | Tasks 9-10 | Medium |
| Phase 4: IUPC Hosting | Tasks 11-12 | Medium |
| Phase 5: Additional | Tasks 13-15 | Low |

**Total: 15 tasks, ~50 steps**

---

## Execution Options

**Plan complete and saved to `docs/superpowers/plans/2026-05-28-bug-fixes-and-features.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
