# EditorialForm Component Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create a reusable React form component for creating editorials with validation, API integration, and error handling.

**Architecture:** Single-file component using individual useState hooks (matching codebase patterns), Tailwind CSS for styling, and the existing api.editorials.create endpoint.

**Tech Stack:** React 19, TypeScript, Tailwind CSS, existing api.ts client

---

## File Structure

| File | Purpose |
|------|---------|
| `web/src/components/EditorialForm.tsx` | **Create** — The form component |
| `web/src/lib/api.ts` | **Read only** — API client (already has editorials.create) |

---

## Task 1: Create EditorialForm Component with Props and State

**Files:**
- Create: `web/src/components/EditorialForm.tsx`

- [ ] **Step 1: Create the component file with props interface and state**

```typescript
// web/src/components/EditorialForm.tsx
import { useState } from 'react'
import { api } from '../lib/api'

interface EditorialFormProps {
  problemId: string
  onSuccess: () => void
  onCancel?: () => void
}

export default function EditorialForm({ problemId, onSuccess, onCancel }: EditorialFormProps) {
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [approach, setApproach] = useState('')
  const [solutionCode, setSolutionCode] = useState('')
  const [solutionLanguage, setSolutionLanguage] = useState('')
  const [timeComplexity, setTimeComplexity] = useState('')
  const [spaceComplexity, setSpaceComplexity] = useState('')
  const [isOfficial, setIsOfficial] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  return (
    <div className="max-w-2xl mx-auto">
      <h2 className="text-2xl font-bold mb-6">Add Editorial</h2>
      {/* Form will be implemented in next task */}
    </div>
  )
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/tahsinarafat/App_Dev/AIOJ/web && npx tsc --noEmit src/components/EditorialForm.tsx`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add web/src/components/EditorialForm.tsx
git commit -m "feat: add EditorialForm component skeleton with props and state"
```

---

## Task 2: Implement Form UI Layout

**Files:**
- Modify: `web/src/components/EditorialForm.tsx`

- [ ] **Step 1: Add complete form UI with all fields**

```typescript
// web/src/components/EditorialForm.tsx
import { useState } from 'react'
import { api } from '../lib/api'

interface EditorialFormProps {
  problemId: string
  onSuccess: () => void
  onCancel?: () => void
}

export default function EditorialForm({ problemId, onSuccess, onCancel }: EditorialFormProps) {
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [approach, setApproach] = useState('')
  const [solutionCode, setSolutionCode] = useState('')
  const [solutionLanguage, setSolutionLanguage] = useState('')
  const [timeComplexity, setTimeComplexity] = useState('')
  const [spaceComplexity, setSpaceComplexity] = useState('')
  const [isOfficial, setIsOfficial] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const canSubmit = !submitting && title.trim() && content.trim()

  return (
    <div className="max-w-2xl mx-auto">
      <h2 className="text-2xl font-bold mb-6">Add Editorial</h2>
      
      {error && (
        <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 px-4 py-2 rounded mb-4 text-sm">
          {error}
        </div>
      )}

      <div className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Title <span className="text-red-500">*</span>
          </label>
          <input
            type="text"
            value={title}
            onChange={e => setTitle(e.target.value)}
            required
            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
            placeholder="Editorial title"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Content <span className="text-red-500">*</span>
          </label>
          <textarea
            value={content}
            onChange={e => setContent(e.target.value)}
            required
            rows={10}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
            placeholder="Write your editorial content..."
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Approach
          </label>
          <textarea
            value={approach}
            onChange={e => setApproach(e.target.value)}
            rows={3}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
            placeholder="Describe your approach..."
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Solution Code
          </label>
          <textarea
            value={solutionCode}
            onChange={e => setSolutionCode(e.target.value)}
            rows={8}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
            placeholder="Paste your solution code here..."
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Solution Language
          </label>
          <input
            type="text"
            value={solutionLanguage}
            onChange={e => setSolutionLanguage(e.target.value)}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
            placeholder="e.g. cpp, python, java"
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Time Complexity
            </label>
            <input
              type="text"
              value={timeComplexity}
              onChange={e => setTimeComplexity(e.target.value)}
              className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
              placeholder="e.g. O(n log n)"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Space Complexity
            </label>
            <input
              type="text"
              value={spaceComplexity}
              onChange={e => setSpaceComplexity(e.target.value)}
              className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
              placeholder="e.g. O(n)"
            />
          </div>
        </div>

        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="isOfficial"
            checked={isOfficial}
            onChange={e => setIsOfficial(e.target.checked)}
            className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
          />
          <label htmlFor="isOfficial" className="text-sm font-medium text-gray-700 dark:text-gray-300">
            Official Editorial
          </label>
        </div>

        <div className="flex gap-3 pt-2">
          <button
            type="button"
            disabled={!canSubmit}
            className="bg-blue-600 text-white px-6 py-2 rounded hover:bg-blue-700 disabled:opacity-50 transition-colors"
          >
            {submitting ? 'Submitting...' : 'Submit Editorial'}
          </button>
          {onCancel && (
            <button
              type="button"
              onClick={onCancel}
              className="bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-6 py-2 rounded hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
            >
              Cancel
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/tahsinarafat/App_Dev/AIOJ/web && npx tsc --noEmit src/components/EditorialForm.tsx`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add web/src/components/EditorialForm.tsx
git commit -m "feat: add EditorialForm UI layout with all fields"
```

---

## Task 3: Implement Form Submission Handler

**Files:**
- Modify: `web/src/components/EditorialForm.tsx`

- [ ] **Step 1: Add handleSubmit function**

```typescript
// web/src/components/EditorialForm.tsx
import { useState } from 'react'
import { api } from '../lib/api'

interface EditorialFormProps {
  problemId: string
  onSuccess: () => void
  onCancel?: () => void
}

export default function EditorialForm({ problemId, onSuccess, onCancel }: EditorialFormProps) {
  const [title, setTitle] = useState('')
  const [content, setContent] = useState('')
  const [approach, setApproach] = useState('')
  const [solutionCode, setSolutionCode] = useState('')
  const [solutionLanguage, setSolutionLanguage] = useState('')
  const [timeComplexity, setTimeComplexity] = useState('')
  const [spaceComplexity, setSpaceComplexity] = useState('')
  const [isOfficial, setIsOfficial] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')

  const canSubmit = !submitting && title.trim() && content.trim()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!canSubmit) return
    
    setSubmitting(true)
    setError('')
    
    try {
      await api.editorials.create({
        problem_id: problemId,
        title: title.trim(),
        content: content.trim(),
        approach: approach.trim() || undefined,
        solution_code: solutionCode.trim() || undefined,
        solution_language: solutionLanguage.trim() || undefined,
        time_complexity: timeComplexity.trim() || undefined,
        space_complexity: spaceComplexity.trim() || undefined,
        is_official: isOfficial,
      })
      onSuccess()
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Failed to create editorial'
      setError(message)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="max-w-2xl mx-auto">
      <h2 className="text-2xl font-bold mb-6">Add Editorial</h2>
      
      {error && (
        <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 px-4 py-2 rounded mb-4 text-sm">
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Title <span className="text-red-500">*</span>
          </label>
          <input
            type="text"
            value={title}
            onChange={e => setTitle(e.target.value)}
            required
            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
            placeholder="Editorial title"
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Content <span className="text-red-500">*</span>
          </label>
          <textarea
            value={content}
            onChange={e => setContent(e.target.value)}
            required
            rows={10}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
            placeholder="Write your editorial content..."
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Approach
          </label>
          <textarea
            value={approach}
            onChange={e => setApproach(e.target.value)}
            rows={3}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
            placeholder="Describe your approach..."
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Solution Code
          </label>
          <textarea
            value={solutionCode}
            onChange={e => setSolutionCode(e.target.value)}
            rows={8}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
            placeholder="Paste your solution code here..."
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Solution Language
          </label>
          <input
            type="text"
            value={solutionLanguage}
            onChange={e => setSolutionLanguage(e.target.value)}
            className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
            placeholder="e.g. cpp, python, java"
          />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Time Complexity
            </label>
            <input
              type="text"
              value={timeComplexity}
              onChange={e => setTimeComplexity(e.target.value)}
              className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
              placeholder="e.g. O(n log n)"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Space Complexity
            </label>
            <input
              type="text"
              value={spaceComplexity}
              onChange={e => setSpaceComplexity(e.target.value)}
              className="w-full border border-gray-300 dark:border-gray-600 rounded-md px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
              placeholder="e.g. O(n)"
            />
          </div>
        </div>

        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="isOfficial"
            checked={isOfficial}
            onChange={e => setIsOfficial(e.target.checked)}
            className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
          />
          <label htmlFor="isOfficial" className="text-sm font-medium text-gray-700 dark:text-gray-300">
            Official Editorial
          </label>
        </div>

        <div className="flex gap-3 pt-2">
          <button
            type="submit"
            disabled={!canSubmit}
            className="bg-blue-600 text-white px-6 py-2 rounded hover:bg-blue-700 disabled:opacity-50 transition-colors"
          >
            {submitting ? 'Submitting...' : 'Submit Editorial'}
          </button>
          {onCancel && (
            <button
              type="button"
              onClick={onCancel}
              className="bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-6 py-2 rounded hover:bg-gray-300 dark:hover:bg-gray-600 transition-colors"
            >
              Cancel
            </button>
          )}
        </div>
      </form>
    </div>
  )
}
```

- [ ] **Step 2: Verify TypeScript compiles**

Run: `cd /Users/tahsinarafat/App_Dev/AIOJ/web && npx tsc --noEmit src/components/EditorialForm.tsx`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add web/src/components/EditorialForm.tsx
git commit -m "feat: add EditorialForm submission handler with validation"
```

---

## Task 4: Final Verification

**Files:**
- Read: `web/src/components/EditorialForm.tsx`

- [ ] **Step 1: Run full TypeScript check**

Run: `cd /Users/tahsinarafat/App_Dev/AIOJ/web && npx tsc --noEmit`
Expected: No errors

- [ ] **Step 2: Verify component exports correctly**

Check that the component is exported as default and can be imported:
```typescript
import EditorialForm from '../components/EditorialForm'
```

- [ ] **Step 3: Final commit with all changes**

```bash
git add web/src/components/EditorialForm.tsx
git commit -m "feat: complete EditorialForm reusable component"
```

---

## Spec Coverage Check

| Spec Requirement | Task |
|------------------|------|
| Props interface (problemId, onSuccess, onCancel) | Task 1 |
| State management (10 useState hooks) | Task 1 |
| Validation (title + content required) | Task 2, 3 |
| API mapping (problem_id, all fields) | Task 3 |
| Error handling (error state, try/catch) | Task 3 |
| Loading state (submitting, disabled button) | Task 2, 3 |
| Cancel button (conditional rendering) | Task 2 |
| UI layout (single-column, Tailwind) | Task 2 |
| No `as any` | All tasks |
| No new dependencies | All tasks |
