# Problem Setter Refactor & Case-Insensitive Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the system-wide role "teacher" to "setter" (Problem Setter), make username authentication and registration case-insensitive, and improve the admin review workflow for setter applications.

**Architecture:**
1. Rename role `'teacher'` to `'setter'` in Go code models, constants, handlers, unit tests, and DB migrations.
2. Update postgres queries in `users.go` and `onsite_users.go` to use `LOWER(username) = LOWER($1)` to ensure case-insensitive matching for authentication and public profile retrieval.
3. Enhance `SetterApplication` model to join `users` and include `username`, and update admin panel UI to display usernames instead of IDs.
4. Relocate the "Setter Workspace" menu link to the profile dropdown menu (accessible to all logged-in users), and display the "Apply to become a Setter" form inside the Setter Panel if a non-setter user visits `/setter`.

**Tech Stack:** Go, PostgreSQL, React, TypeScript, Tailwind CSS

---

### Task 1: DB Schema and Migration Updates

**Files:**
- Modify: `internal/store/migrations/000001_init.up.sql`
- Modify: `internal/store/migrations/000047_contestant_role.up.sql`
- Modify: `internal/store/migrations/000047_contestant_role.down.sql`
- Create: `internal/store/migrations/000048_rename_teacher_to_setter.up.sql`
- Create: `internal/store/migrations/000048_rename_teacher_to_setter.down.sql`

- [ ] **Step 1: Edit 000001_init.up.sql**

Change `'teacher'` to `'setter'` in `users_role_check` constraint definition.

- [ ] **Step 2: Edit 000047_contestant_role.up.sql**

Change `'teacher'` to `'setter'` in the altered `users_role_check` constraint definition.

- [ ] **Step 3: Edit 000047_contestant_role.down.sql**

Change `'teacher'` to `'setter'` in the rolled-back `users_role_check` constraint definition.

- [ ] **Step 4: Create 000048_rename_teacher_to_setter.up.sql**

Create a new migration file to alter the constraint and update existing records:

```sql
-- Remove old check constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
-- Add new check constraint with 'setter' instead of 'teacher'
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin','setter','user','bot','contestant'));
-- Update any existing 'teacher' role users to 'setter'
UPDATE users SET role = 'setter' WHERE role = 'teacher';
```

- [ ] **Step 5: Create 000048_rename_teacher_to_setter.down.sql**

Create down migration to rollback:

```sql
-- Remove constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
-- Add old check constraint with 'teacher'
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin','teacher','user','bot','contestant'));
-- Update back
UPDATE users SET role = 'teacher' WHERE role = 'setter';
```

- [ ] **Step 6: Run migrations to test**

Run: `make migrate-down && make migrate-up`
Expected: PASS without errors.

- [ ] **Step 7: Commit DB migration updates**

```bash
git add internal/store/migrations/
git commit -m "migration: rename teacher role to setter and update constraints"
```

---

### Task 2: Go Backend Role Refactoring

**Files:**
- Modify: `internal/model/constants.go`
- Modify: `internal/model/model_test.go`
- Modify: `internal/api/handler/admin.go`
- Modify: `internal/api/handler/contest.go`
- Modify: `internal/api/handler/import.go`
- Modify: `internal/api/handler/import_test.go`

- [ ] **Step 1: Edit internal/model/constants.go**

Change `RoleTeacher = "teacher"` to `RoleSetter = "setter"`.

- [ ] **Step 2: Edit internal/model/model_test.go**

Change `RoleTeacher` to `RoleSetter`.

- [ ] **Step 3: Edit internal/api/handler/admin.go**

Update validRoles map to include `setter` instead of `teacher` (around line 42), and update `UpdateUserRole` and `ReviewSetterApp` to assign role `"setter"` instead of `"teacher"`.

- [ ] **Step 4: Edit internal/api/handler/contest.go**

Update role checks from `claims.Role != "teacher"` to `claims.Role != "setter"`.

- [ ] **Step 5: Edit internal/api/handler/import.go**

Update role checks from `claims.Role != "teacher"` to `claims.Role != "setter"`.

- [ ] **Step 6: Edit internal/api/handler/import_test.go**

Update test claims to use `"setter"` role and rename `"teacher role - allowed"` test names to `"setter role - allowed"`.

- [ ] **Step 7: Verify backend builds and tests pass**

Run: `go test ./internal/...`
Expected: PASS

- [ ] **Step 8: Commit role changes**

```bash
git add internal/model/ internal/api/
git commit -m "refactor: rename teacher role to setter in backend handlers and tests"
```

---

### Task 3: Case-Insensitive Username Authentication

**Files:**
- Modify: `internal/store/postgres/users.go`
- Modify: `internal/store/postgres/onsite_users.go`

- [ ] **Step 1: Modify internal/store/postgres/users.go**

Update `GetByUsername` to query with `LOWER(username) = LOWER($1)`:

```go
func (s *UserStore) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id,username,email,password_hash,role,is_bot,created_at,updated_at FROM users WHERE LOWER(username)=LOWER($1)`,
		username).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role, &u.IsBot, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
```

Update `GetPublicProfile` where clause to use `WHERE LOWER(u.username) = LOWER($1)`.

- [ ] **Step 2: Modify internal/store/postgres/onsite_users.go**

Update `GetByUsername` to query with `LOWER(username) = LOWER($1)`:

```go
func (s *OnsiteUserStoreImpl) GetByUsername(ctx context.Context, username string) (*model.OnsiteBatchUser, error) {
	var u model.OnsiteBatchUser
	err := s.db.QueryRowContext(ctx,
		`SELECT id, contest_id, team_name, institution, username, password_hash, is_used, used_by, created_at
		 FROM onsite_batch_users WHERE LOWER(username) = LOWER($1)`,
		username,
	).Scan(&u.ID, &u.ContestID, &u.TeamName, &u.Institution, &u.Username, &u.PasswordHash, &u.IsUsed, &u.UsedBy, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get by username: %w", err)
	}
	return &u, nil
}
```

- [ ] **Step 3: Run backend tests**

Run: `go test ./internal/...`
Expected: PASS

- [ ] **Step 4: Commit case-insensitivity updates**

```bash
git add internal/store/postgres/
git commit -m "feat: make user lookup and authentication case-insensitive"
```

---

### Task 4: Enhance Setter Application Review with Username

**Files:**
- Modify: `internal/model/user.go`
- Modify: `internal/store/postgres/setter.go`
- Modify: `web/src/pages/admin/SetterAppsPanel.tsx`

- [ ] **Step 1: Modify internal/model/user.go**

Add `Username` field to `SetterApplication` struct:

```go
type SetterApplication struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 2: Modify internal/store/postgres/setter.go**

Update `ListApplications` to join the `users` table and retrieve `username`:

```go
func (s *SetterStore) ListApplications(ctx context.Context) ([]model.SetterApplication, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT sa.user_id, u.username, sa.status, sa.reason, sa.created_at 
		 FROM setter_applications sa 
		 JOIN users u ON u.id = sa.user_id 
		 ORDER BY sa.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []model.SetterApplication
	for rows.Next() {
		var a model.SetterApplication
		if err := rows.Scan(&a.UserID, &a.Username, &a.Status, &a.Reason, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	if items == nil {
		items = []model.SetterApplication{}
	}
	return items, nil
}
```

Update `GetApplication` in the same way to join and scan `sa.user_id, u.username, sa.status, sa.reason, sa.created_at`.

- [ ] **Step 3: Modify web/src/pages/admin/SetterAppsPanel.tsx**

Update the headers and rows to show Applicant Username:

Change header:
```html
<th className="px-4 py-3 text-left">Applicant</th>
```
Change data cell:
```html
<td className="px-4 py-3 font-semibold">{a.username}</td>
```

- [ ] **Step 4: Verify frontend builds and admin tests**

Run: `cd web && npm run build`
Expected: PASS

- [ ] **Step 5: Commit application review enhancement**

```bash
git add internal/model/internal/store/postgres/ web/src/pages/admin/
git commit -m "feat: show applicant usernames in admin review panel"
```

---

### Task 5: Relocate Setter Access Links & Request Form

**Files:**
- Create: `web/src/components/SetterApplication.tsx`
- Modify: `web/src/components/Navbar.tsx`
- Modify: `web/src/pages/Profile.tsx`
- Modify: `web/src/pages/SetterPanel.tsx`
- Modify: `web/src/pages/ContestPlagiarism.tsx`
- Modify: `web/src/pages/OrganizationList.tsx`
- Modify: `web/src/pages/ProblemDetail.tsx`
- Modify: `web/src/pages/ProblemList.tsx`
- Modify: `web/src/pages/TrainingPlanList.tsx`
- Modify: `web/src/pages/admin/UsersPanel.tsx`

- [ ] **Step 1: Create web/src/components/SetterApplication.tsx**

Move the application form logic from `Profile.tsx` to a reusable component:

```typescript
import { useEffect, useState } from 'react'
import { api } from '../lib/api'

export default function SetterApplication() {
    const [status, setStatus] = useState<string | null>(null)
    const [reason, setReason] = useState('')
    const [submitted, setSubmitted] = useState(false)
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        api.setter.status().then(d => setStatus(d?.status || null)).catch(() => {}).finally(() => setLoading(false))
    }, [])

    const handleApply = async () => {
        try {
            await api.setter.apply(reason)
            setSubmitted(true)
            setStatus('pending')
        } catch (e: any) {
            alert('Failed: ' + e.message)
        }
    }

    if (loading) return <p className="text-sm text-gray-400 dark:text-gray-500">Loading...</p>
    if (status === 'approved') return <p className="text-green-600 dark:text-green-400 text-sm">You are a problem setter!</p>
    if (status === 'pending') return <p className="text-yellow-600 dark:text-yellow-400 text-sm">Your application is pending review.</p>

    return (
        <div>
            {status === 'rejected' && <p className="text-red-600 dark:text-red-400 text-sm mb-2">Your previous application was rejected. You can re-apply.</p>}
            {!submitted ? (
                <div>
                    <textarea rows={3} value={reason} onChange={e => setReason(e.target.value)}
                        placeholder="Why do you want to become a setter?"
                        className="w-full border rounded px-3 py-2 text-sm mb-2 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100" />
                    <button onClick={handleApply} disabled={!reason.trim()}
                        className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 disabled:opacity-50 transition-colors">
                        Apply as Problem Setter
                    </button>
                </div>
            ) : (
                <p className="text-green-600 dark:text-green-400 text-sm">Application submitted!</p>
            )}
        </div>
    )
}
```

- [ ] **Step 2: Modify web/src/components/Navbar.tsx**

1. Change `isSetter` to look for `"setter"`:
```typescript
const isSetter = role === 'setter' || isAdmin
```
2. Remove the `<Link to="/setter">` from the horizontal desktop menu.
3. In `NavDropdown` for User Profile, add:
```typescript
<NavLink to="/setter" icon={FileCode}>Setter Workspace</NavLink>
```
4. In mobile drawer menu, render "Setter Workspace" for all logged in users.

- [ ] **Step 3: Modify web/src/pages/Profile.tsx**

1. Import `SetterApplication` from `../components/SetterApplication`.
2. Change `user.role === 'teacher'` checks to `user.role === 'setter'`.
3. Check `user.role === 'user'` to show `<SetterApplication />`.

- [ ] **Step 4: Modify web/src/pages/SetterPanel.tsx**

1. Update `const isSetter = role === 'setter' || role === 'admin'`.
2. Import `SetterApplication` from `../components/SetterApplication`.
3. If `!isSetter`, render the `SetterApplication` component inside a card:

```typescript
    if (!isSetter) {
        return (
            <div className="max-w-md mx-auto mt-10 p-6 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-sm">
                <h2 className="text-xl font-bold mb-2 text-gray-900 dark:text-gray-100">Become a Problem Setter</h2>
                <p className="text-gray-600 dark:text-gray-400 text-sm mb-6">
                    Problem setters can create problems and host contests on AIOJ. Request access below by providing a reason.
                </p>
                <SetterApplication />
            </div>
        )
    }
```

- [ ] **Step 5: Modify other frontend files**

1. Update all other occurrences of `"teacher"` role check in `ContestPlagiarism.tsx`, `OrganizationList.tsx`, `ProblemDetail.tsx`, `ProblemList.tsx`, `TrainingPlanList.tsx`, and `admin/UsersPanel.tsx` to `"setter"`.

- [ ] **Step 6: Build and test frontend**

Run: `cd web && npm run build`
Expected: PASS without typescript or build errors.

- [ ] **Step 7: Commit relocation updates**

```bash
git add web/
git commit -m "feat: move setter workspace link to dropdown and support request form in workspace panel"
```
