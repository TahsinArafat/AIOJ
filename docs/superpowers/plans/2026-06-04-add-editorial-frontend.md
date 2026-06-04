# Direct Editorial Creation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create frontend user interfaces in the Problem Setter workspace and on the public Problem Detail page to allow Admins, Judges, and Collaborators to create editorials for problems.

**Architecture:** A reusable `EditorialForm` component manages local form state, client validation, and API submission. The Setter Workspace embeds this form in a new sidebar tab. The public Problem Detail page renders a modal containing this form for users with permission.

**Tech Stack:** React 19, TypeScript, Tailwind CSS, Vite

---

### Task 1: Create Reusable Editorial Form Component

**Files:**
- Create: `/Users/tahsinarafat/App_Dev/AIOJ/web/src/components/EditorialForm.tsx`

- [ ] **Step 1: Write the EditorialForm component**

```tsx
import React, { useState } from 'react'
import { api } from '../lib/api'

interface EditorialFormProps {
  problemId: string
  isUserAdmin: boolean
  onSuccess: () => void
  onCancel?: () => void
}

export default function EditorialForm({ problemId, isUserAdmin, onSuccess, onCancel }: EditorialFormProps) {
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [approach, setApproach] = useState('')
  const [solutionCode, setSolutionCode] = useState('')
  const [solutionLanguage, setSolutionLanguage] = useState('cpp')
  const [timeComplexity, setTimeComplexity] = useState('')
  const [spaceComplexity, setSpaceComplexity] = useState('')
  const [isOfficial, setIsOfficial] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim() || !content.trim()) {
      setError('Title and Content are required.')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      await api.editorials.create({
        problem_id: problemId,
        title: title.trim(),
        content: content.trim(),
        approach: approach.trim() || undefined,
        solution_code: solutionCode.trim() || undefined,
        solution_language: solutionLanguage || undefined,
        time_complexity: timeComplexity.trim() || undefined,
        space_complexity: spaceComplexity.trim() || undefined,
        is_official: isUserAdmin ? isOfficial : false,
      })
      onSuccess()
    } catch (err: any) {
      setError(err.message || 'Failed to submit editorial.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4 text-sm">
      {error && <div className="p-3 bg-red-50 text-red-700 rounded border border-red-200">{error}</div>}
      
      <div>
        <label className="block font-medium text-gray-700 dark:text-gray-300 mb-1">Title *</label>
        <input
          type="text"
          value={title}
          onChange={e => setTitle(e.target.value)}
          required
          className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-800 dark:border-gray-700"
          placeholder="e.g. Optimal Greedy Approach"
        />
      </div>

      <div>
        <label className="block font-medium text-gray-700 dark:text-gray-300 mb-1">Approach Description</label>
        <textarea
          value={approach}
          onChange={e => setApproach(e.target.value)}
          rows={3}
          className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-800 dark:border-gray-700"
          placeholder="Briefly summarize the logic/approach..."
        />
      </div>

      <div>
        <label className="block font-medium text-gray-700 dark:text-gray-300 mb-1">Editorial Content (Markdown Supported) *</label>
        <textarea
          value={content}
          onChange={e => setContent(e.target.value)}
          rows={6}
          required
          className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-800 dark:border-gray-700 font-mono"
          placeholder="Detailed explanation, proofs, cases..."
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block font-medium text-gray-700 dark:text-gray-300 mb-1">Time Complexity</label>
          <input
            type="text"
            value={timeComplexity}
            onChange={e => setTimeComplexity(e.target.value)}
            className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-800 dark:border-gray-700"
            placeholder="e.g. O(N log N)"
          />
        </div>
        <div>
          <label className="block font-medium text-gray-700 dark:text-gray-300 mb-1">Space Complexity</label>
          <input
            type="text"
            value={spaceComplexity}
            onChange={e => setSpaceComplexity(e.target.value)}
            className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-800 dark:border-gray-700"
            placeholder="e.g. O(N)"
          />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block font-medium text-gray-700 dark:text-gray-300 mb-1">Solution Language</label>
          <select
            value={solutionLanguage}
            onChange={e => setSolutionLanguage(e.target.value)}
            className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-800 dark:border-gray-700"
          >
            <option value="cpp">C++</option>
            <option value="python">Python</option>
            <option value="java">Java</option>
            <option value="go">Go</option>
            <option value="rust">Rust</option>
            <option value="javascript">JavaScript</option>
          </select>
        </div>
      </div>

      <div>
        <label className="block font-medium text-gray-700 dark:text-gray-300 mb-1">Solution Code</label>
        <textarea
          value={solutionCode}
          onChange={e => setSolutionCode(e.target.value)}
          rows={6}
          className="w-full border rounded px-3 py-2 bg-white dark:bg-gray-800 dark:border-gray-700 font-mono"
          placeholder="Paste clean solution code here..."
        />
      </div>

      {isUserAdmin && (
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="isOfficial"
            checked={isOfficial}
            onChange={e => setIsOfficial(e.target.checked)}
            className="rounded border-gray-300 dark:border-gray-700"
          />
          <label htmlFor="isOfficial" className="font-medium text-gray-700 dark:text-gray-300">Mark as Official Editorial</label>
        </div>
      )}

      <div className="flex gap-2 justify-end pt-4">
        {onCancel && (
          <button
            type="button"
            onClick={onCancel}
            disabled={submitting}
            className="border px-4 py-2 rounded text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
          >
            Cancel
          </button>
        )}
        <button
          type="submit"
          disabled={submitting}
          className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded font-medium disabled:opacity-50"
        >
          {submitting ? 'Submitting...' : 'Save Editorial'}
        </button>
      </div>
    </form>
  )
}
```

- [ ] **Step 2: Commit file**

```bash
git add /Users/tahsinarafat/App_Dev/AIOJ/web/src/components/EditorialForm.tsx
git commit -m "feat: add reusable EditorialForm component"
```

---

### Task 2: Create AddEditorialModal for Inline Creation

**Files:**
- Create: `/Users/tahsinarafat/App_Dev/AIOJ/web/src/components/AddEditorialModal.tsx`

- [ ] **Step 1: Write the AddEditorialModal component**

```tsx
import React from 'react'
import EditorialForm from './EditorialForm'

interface AddEditorialModalProps {
  problemId: string
  isUserAdmin: boolean
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

export default function AddEditorialModal({ problemId, isUserAdmin, isOpen, onClose, onSuccess }: AddEditorialModalProps) {
  if (!isOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 overflow-y-auto">
      <div className="relative w-full max-w-2xl bg-white dark:bg-gray-800 rounded-lg shadow-lg max-h-[90vh] overflow-y-auto p-6">
        <div className="flex justify-between items-center border-b pb-3 mb-4">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Add Editorial</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200">
            ✕
          </button>
        </div>
        <EditorialForm
          problemId={problemId}
          isUserAdmin={isUserAdmin}
          onSuccess={() => {
            onSuccess()
            onClose()
          }}
          onCancel={onClose}
        />
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Commit file**

```bash
git add /Users/tahsinarafat/App_Dev/AIOJ/web/src/components/AddEditorialModal.tsx
git commit -m "feat: add AddEditorialModal wrapper"
```

---

### Task 3: Create Workspace EditorialTab Component

**Files:**
- Create: `/Users/tahsinarafat/App_Dev/AIOJ/web/src/components/SetterWorkspace/EditorialTab.tsx`

- [ ] **Step 1: Write EditorialTab for Setter Workspace**

```tsx
import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import EditorialForm from '../EditorialForm'

interface EditorialTabProps {
  problemId: string
  isUserAdmin: boolean
}

export default function EditorialTab({ problemId, isUserAdmin }: EditorialTabProps) {
  const [editorials, setEditorials] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [isAdding, setIsAdding] = useState(false)

  const loadEditorials = async () => {
    try {
      const res = await api.editorials.getByProblem(problemId)
      setEditorials(res.data || [])
    } catch (err) {
      console.error('Failed to load editorials', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadEditorials()
  }, [problemId])

  if (loading) {
    return <div className="text-center py-10 text-gray-500">Loading editorials...</div>
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center border-b pb-4">
        <div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-gray-100">Editorials</h2>
          <p className="text-xs text-gray-500">Create or view explanation of solutions for this problem.</p>
        </div>
        {!isAdding && (
          <button
            onClick={() => setIsAdding(true)}
            className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1.5 rounded text-sm font-medium"
          >
            Add Editorial
          </button>
        )}
      </div>

      {isAdding ? (
        <div className="border rounded-lg p-6 bg-gray-50 dark:bg-gray-900/50">
          <h3 className="text-md font-semibold mb-4 text-gray-900 dark:text-gray-100">New Editorial</h3>
          <EditorialForm
            problemId={problemId}
            isUserAdmin={isUserAdmin}
            onSuccess={() => {
              setIsAdding(false)
              loadEditorials()
            }}
            onCancel={() => setIsAdding(false)}
          />
        </div>
      ) : editorials.length === 0 ? (
        <div className="text-center py-20 border-2 border-dashed rounded-lg text-gray-400 dark:text-gray-500">
          No editorials created yet.
        </div>
      ) : (
        <div className="grid gap-4">
          {editorials.map(e => (
            <div key={e.id} className="border rounded-lg p-4 bg-white dark:bg-gray-800 flex justify-between items-center">
              <div>
                <div className="flex items-center gap-2">
                  {e.is_official && (
                    <span className="text-[10px] font-bold uppercase bg-green-100 dark:bg-green-900/40 text-green-700 dark:text-green-300 px-1.5 py-0.5 rounded">
                      Official
                    </span>
                  )}
                  <h4 className="font-semibold text-gray-900 dark:text-gray-100">{e.title}</h4>
                </div>
                <div className="text-xs text-gray-400 mt-1">
                  By {e.username} • upvotes: {e.upvotes}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
```

- [ ] **Step 2: Commit file**

```bash
git add /Users/tahsinarafat/App_Dev/AIOJ/web/src/components/SetterWorkspace/EditorialTab.tsx
git commit -m "feat: add Setter Workspace EditorialTab"
```

---

### Task 4: Integrate Editorial Tab in Setter Workspace View

**Files:**
- Modify: `/Users/tahsinarafat/App_Dev/AIOJ/web/src/pages/SetterProblemWorkspace.tsx`

- [ ] **Step 1: Import EditorialTab and adjust type definitions**

Add `'editorial'` to `WorkspaceTab` type (line ~15):
```typescript
type WorkspaceTab = 'statement' | 'testcases' | 'checker' | 'permissions' | 'settings' | 'editorial'
```

Update tab values inclusion list (line ~19):
```typescript
const validTabs: WorkspaceTab[] = ['statement', 'testcases', 'checker', 'permissions', 'settings', 'editorial']
```

- [ ] **Step 2: Add navigation buttons & conditional tabs rendering**

Add Sidebar navigation button in the list of tabs:
```tsx
<button
  onClick={() => setActiveTab('editorial')}
  className={`px-4 py-3 text-left border-b border-gray-100 dark:border-gray-700 font-medium ${activeTab === 'editorial' ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 border-l-4 border-l-blue-600' : 'text-gray-600 dark:text-gray-400 hover:bg-gray-50 dark:hover:bg-gray-700 hover:text-black dark:hover:text-white'}`}
>
  Editorials
</button>
```

Add Tab content render (ensure user admin role detection or pass user validation if user role check logic exists):
```tsx
{activeTab === 'editorial' && (
  <EditorialTab
    problemId={problem.id}
    isUserAdmin={true} // Hardcode or check user context if role is checked
  />
)}
```
*Note: Make sure to import `EditorialTab` at top of the file:*
```typescript
import EditorialTab from '../components/SetterWorkspace/EditorialTab'
```

- [ ] **Step 3: Commit file changes**

```bash
git add /Users/tahsinarafat/App_Dev/AIOJ/web/src/pages/SetterProblemWorkspace.tsx
git commit -m "feat: integrate Editorial Tab in Setter Problem Workspace"
```

---

### Task 5: Add Inline Editorial creation inside Public Problem detail tab

**Files:**
- Modify: `/Users/tahsinarafat/App_Dev/AIOJ/web/src/pages/ProblemDetail.tsx`

- [ ] **Step 1: Import AddEditorialModal**

Import the modal component at the top of the file:
```typescript
import AddEditorialModal from '../components/AddEditorialModal'
```

- [ ] **Step 2: Add modal state and check user permissions**

Define a state for modal visibility and permissions check:
```typescript
const [isAddModalOpen, setIsAddModalOpen] = useState(false)
const [hasSetterPermissions, setHasSetterPermissions] = useState(false)

// In useEffect / loading problem hook, call API or check permissions:
useEffect(() => {
  if (problem) {
    api.problems.getPermissions(problem.slug)
      .then(res => {
        // If user is owner/tester/co-author, allow editorial direct creation
        setHasSetterPermissions(res.data && res.data.length > 0)
      })
      .catch(() => setHasSetterPermissions(false))
  }
}, [problem])
```

- [ ] **Step 3: Render Add button & Modal**

Modify the editorials tab view (line ~584):
```tsx
) : tab === 'editorials' ? (
    <div className="space-y-4">
        {hasSetterPermissions && (
            <div className="flex justify-end mb-2">
                <button
                    onClick={() => setIsAddModalOpen(true)}
                    className="bg-blue-600 hover:bg-blue-700 text-white px-3 py-1.5 rounded text-xs font-medium"
                >
                    Add Editorial
                </button>
            </div>
        )}
        {editorials.length === 0 ? (
            <p className="text-gray-400 dark:text-gray-500 text-sm">No editorials yet for this problem.</p>
        ) : (
            editorials.map(e => (
                <Link key={e.id} to={`/editorials/${e.id}`} className="block border rounded p-4 hover:bg-gray-50 dark:hover:bg-gray-700">
                    <div className="flex items-center gap-2 mb-1">
                        {e.is_official && <span className="text-xs bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 px-2 py-0.5 rounded font-medium">Official</span>}
                        <h4 className="font-medium">{e.title}</h4>
                    </div>
                    <div className="flex gap-4 text-xs text-gray-400 dark:text-gray-500">
                        <span>{e.username}</span>
                        {e.time_complexity && <span>Time: {e.time_complexity}</span>}
                        <span>{e.upvotes} upvotes</span>
                    </div>
                </Link>
            ))
        )}

        <AddEditorialModal
            problemId={problem.id}
            isUserAdmin={hasSetterPermissions}
            isOpen={isAddModalOpen}
            onClose={() => setIsAddModalOpen(false)}
            onSuccess={() => {
                // reload editorials
                api.editorials.getByProblem(problem.id).then(res => setEditorials(res.data || []))
            }}
        />
    </div>
)
```

- [ ] **Step 4: Commit file changes**

```bash
git add /Users/tahsinarafat/App_Dev/AIOJ/web/src/pages/ProblemDetail.tsx
git commit -m "feat: enable inline editorial creation from Problem Detail view"
```
