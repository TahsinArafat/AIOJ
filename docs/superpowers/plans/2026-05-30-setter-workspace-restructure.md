# Setter Problem Workspace Restructure — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restructure the 1101-line monolithic `SetterProblemWorkspace.tsx` into an orchestrator + 5 focused tab sub-components with shared types, eliminating 5x payload duplication and adding type safety.

**Architecture:** One orchestrator component owns all form state and the `buildPayload()` utility. Five tab sub-components receive sliced props and render independently. All save operations flow through the single `buildPayload()` + `api.problems.update()` path in the orchestrator.

**Tech Stack:** React 19, TypeScript 6.0, Tailwind CSS 4, Vitest, Testing Library, `@/` path alias (maps to `web/src/`).

**Critical Constraints:**
- `noUnusedLocals: true`, `noUnusedParameters: true` in tsconfig — zero unused vars/params
- `verbatimModuleSyntax: true` — must use `import type` for type-only imports
- Strictly preserve existing styling (blue-600 active, white bg, border-gray-200, gray-50 tables, gray-800 buttons)
- Zero compilation errors after each wave

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `web/src/types/problem-workspace.ts` | **Create** | Shared interfaces: `ProblemFormState`, `TestCase`, `Collaborator` |
| `web/src/components/SetterWorkspace/StatementTab.tsx` | **Create** | Statement form: title, description, limits, tags, sample cases |
| `web/src/components/SetterWorkspace/TestCasesTab.tsx` | **Create** | Test case upload, score table, batch set, register match |
| `web/src/components/SetterWorkspace/CheckerTab.tsx` | **Create** | Checker type, epsilon, SPJ editor with presets, interactive toggle |
| `web/src/components/SetterWorkspace/PermissionsTab.tsx` | **Create** | Collaborators table, add/remove collaborator |
| `web/src/components/SetterWorkspace/SettingsTab.tsx` | **Create** | Visibility toggle, danger zone delete |
| `web/src/pages/SetterProblemWorkspace.tsx` | **Modify** | Reduced to orchestrator (~200 lines), uses sub-components |
| `web/src/pages/SetterProblemWorkspace.test.tsx` | **Create** | Component tests for each tab |

---

## Scenario Contract Definition

Every task uses this TDD contract format:

```
### Scenario Contract
Given <initial state>
When  <user action / system event>
Then  <expected behavior>

RED:    <command that should fail>
GREEN:  <implementation that makes it pass>
SURFACE:<verification that it works>
```

---

## WAVE 0: Foundation — Types + buildPayload() + Directory

### Task 0.1: Create types file and SetterWorkspace directory

**Files:**
- Create: `web/src/types/problem-workspace.ts`
- Create: `web/src/components/SetterWorkspace/` (directory)

### Scenario Contract
| Given | When | Then |
|-------|------|------|
| Types file doesn't exist | Creating `problem-workspace.ts` | All 3 interfaces are exported and importable |
| Component dir doesn't exist | Creating directory | Directory exists for tab components |

- [ ] **Step 1: Create the types file**

Write `web/src/types/problem-workspace.ts`:

```typescript
export interface TestCase {
  input_name: string
  output_name: string
  score: number
}

export interface Collaborator {
  problem_id: string
  user_id: string
  username: string
  access_level: string
}

export interface ProblemFormState {
  // Statement
  title: string
  description: string
  inputFormat: string
  outputFormat: string
  hint: string
  timeLimit: number
  memoryLimit: number
  difficulty: string
  tags: string
  sampleCases: { input: string; output: string; explanation: string }[]

  // Test Cases
  testcases: TestCase[]

  // Checker
  checkerType: string
  floatEpsilon: number
  spj: boolean
  spjLanguage: string
  spjSourceCode: string

  // Interactive
  interactive: boolean
  interactorLanguage: string
  interactorSourceCode: string

  // Settings
  visible: boolean
}

export function createDefaultFormState(): ProblemFormState {
  return {
    title: '',
    description: '',
    inputFormat: '',
    outputFormat: '',
    hint: '',
    timeLimit: 1000,
    memoryLimit: 262144,
    difficulty: 'easy',
    tags: '',
    sampleCases: [],
    testcases: [],
    checkerType: 'exact',
    floatEpsilon: 1e-6,
    spj: false,
    spjLanguage: 'cpp-gpp-64',
    spjSourceCode: '',
    interactive: false,
    interactorLanguage: 'cpp-gpp-64',
    interactorSourceCode: '',
    visible: true,
  }
}
```

- [ ] **Step 2: Verify TypeScript compilation**

Run: `npx tsc -b --noEmit`
Expected: No errors (the import isn't used yet, but the file itself should compile — it has no imports and exports only types+functions).

If there's a compile error about unused file or module detection, verify the file is included by `tsconfig.app.json` which has `"include": ["src"]` — any `.ts` under `src/` is included.

- [ ] **Step 3: Create the directory**

Run:
```bash
mkdir -p web/src/components/SetterWorkspace
touch web/src/components/SetterWorkspace/.gitkeep
```

- [ ] **Step 4: Commit**

```bash
git add web/src/types/problem-workspace.ts web/src/components/SetterWorkspace/.gitkeep
git commit -m "feat(workspace): add ProblemFormState types and SetterWorkspace directory"
```

---

### Task 0.2: Write the SetterWorkspace test infrastructure

**Files:**
- Create: `web/src/pages/SetterProblemWorkspace.test.tsx`

### Scenario Contract
| Given | When | Then |
|-------|------|------|
| Test file exists | Running `vitest run` | Tests fail (component not yet restructured) |

- [ ] **Step 1: Write the test file for the workspace**

Write `web/src/pages/SetterProblemWorkspace.test.tsx`:

```typescript
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { expect, test, vi, beforeEach } from 'vitest'
import SetterProblemWorkspace from './SetterProblemWorkspace'
import { api } from '../lib/api'
import type { Mock } from 'vitest'

vi.mock('../lib/api', () => ({
  api: {
    problems: {
      get: vi.fn(),
      update: vi.fn().mockResolvedValue({}),
      getPermissions: vi.fn().mockResolvedValue({ data: [] }),
      delete: vi.fn().mockResolvedValue({}),
    },
  },
  getAccessToken: vi.fn().mockReturnValue('mock-token'),
}))

const mockProblem = {
  id: 'prob-1',
  slug: 'test-problem',
  title: 'Test Problem',
  description: '# Description',
  input_format: 'Two integers',
  output_format: 'Sum',
  hint: 'Think harder',
  time_limit: 1000,
  memory_limit: 262144,
  difficulty: 'easy',
  tags: ['math', 'ad-hoc'],
  sample_cases: [{ input: '1 2', output: '3', explanation: '' }],
  testcase_score: [],
  checker_type: 'exact',
  float_epsilon: 1e-6,
  spj: false,
  spj_language: 'cpp-gpp-64',
  spj_source_code: '',
  interactive: false,
  interactor_language: 'cpp-gpp-64',
  interactor_source_code: '',
  visible: true,
}

beforeEach(() => {
  vi.clearAllMocks()
  ;(api.problems.get as Mock).mockResolvedValue(mockProblem)
})

function renderWorkspace() {
  return render(
    <MemoryRouter initialEntries={['/setter/problem/test-problem']}>
      <Routes>
        <Route path="/setter/problem/:slug" element={<SetterProblemWorkspace />} />
      </Routes>
    </MemoryRouter>
  )
}

test('loads and displays problem title in header', async () => {
  renderWorkspace()
  await waitFor(() => {
    expect(screen.getByText('Test Problem')).toBeInTheDocument()
  })
})

test('renders all 5 tab buttons', async () => {
  renderWorkspace()
  await waitFor(() => {
    expect(screen.getByText('Statement & Details')).toBeInTheDocument()
    expect(screen.getByText('Test Cases / Data')).toBeInTheDocument()
    expect(screen.getByText('Checker / Special Judge')).toBeInTheDocument()
    expect(screen.getByText('Collaborators')).toBeInTheDocument()
    expect(screen.getByText('Workspace Settings')).toBeInTheDocument()
  })
})

test('switches tabs when clicked', async () => {
  renderWorkspace()
  await waitFor(() => {
    expect(screen.getByText('Test Problem')).toBeInTheDocument()
  })

  // Click test cases tab
  fireEvent.click(screen.getByText('Test Cases / Data'))
  await waitFor(() => {
    expect(screen.getByText('Upload Testcase Package (ZIP)')).toBeInTheDocument()
  })

  // Click checker tab
  fireEvent.click(screen.getByText('Checker / Special Judge'))
  await waitFor(() => {
    expect(screen.getByText('Checker & Special Judge Configuration')).toBeInTheDocument()
  })

  // Click collaborators tab
  fireEvent.click(screen.getByText('Collaborators'))
  await waitFor(() => {
    expect(screen.getByText('Problem Collaborators')).toBeInTheDocument()
  })

  // Click settings tab
  fireEvent.click(screen.getByText('Workspace Settings'))
  await waitFor(() => {
    expect(screen.getByText('Problem Visibility')).toBeInTheDocument()
  })
})

test('save statement calls api.problems.update with correct payload', async () => {
  renderWorkspace()
  await waitFor(() => {
    expect(screen.getByText('Test Problem')).toBeInTheDocument()
  })

  const saveBtn = screen.getByText('Save Statement')
  fireEvent.click(saveBtn)

  await waitFor(() => {
    expect(api.problems.update).toHaveBeenCalledTimes(1)
    expect(api.problems.update).toHaveBeenCalledWith('test-problem', expect.objectContaining({
      title: 'Test Problem',
      difficulty: 'easy',
      time_limit: 1000,
    }))
  })
})

test('shows error banner on API failure', async () => {
  ;(api.problems.get as Mock).mockRejectedValue(new Error('Network error'))
  renderWorkspace()
  await waitFor(() => {
    expect(screen.getByText('Network error')).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Run tests to verify they fail (RED)**

Run: `npx vitest run src/pages/SetterProblemWorkspace.test.tsx --reporter=verbose`
Expected: Tests fail because the component currently doesn't support router matching `/setter/problem/:slug` — the component expects `:slug` param but might render immediately or throw. Note the failure mode.

Actually, the current component uses `useParams()` with key `slug`, and in `MemoryRouter` the route pattern needs to match. The current component works with `/setter/problem/:slug` path. Let me verify this... The component just reads `slug`, it doesn't check the path. So the test should match successfully.

But the key thing the test will catch is: the existing component (1101 lines) will render, and the tests test behavior. Since we're not changing behavior, the tests should pass currently. This means we need to run the tests BEFORE changes to get a baseline, then the restructured version should still pass.

Actually, this test file will exist before restructure, so the tests should pass against the current implementation. That's the baseline.

- [ ] **Step 3: Run tests to establish baseline**

Run: `npx vitest run src/pages/SetterProblemWorkspace.test.tsx --reporter=verbose`
Expected: ALL TESTS PASS against current implementation. This establishes the behavior baseline.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/SetterProblemWorkspace.test.tsx
git commit -m "test(workspace): add baseline tests before restructure"
```

---

### Task 0.3: Extract buildPayload() and consolidate state into ProblemFormState

**Files:**
- Modify: `web/src/pages/SetterProblemWorkspace.tsx`

### Scenario Contract
| Given | When | Then |
|-------|------|------|
| 5 handlers each build the same payload | Extract `buildPayload()` | Single function returns same shape, all 5 handlers call it |
| 25+ flat `useState` calls | Consolidate into single `ProblemFormState` | All fields accessible via `formState.field` |
| All tests pass before | Run tests | Same tests still pass (behavior unchanged) |

- [ ] **Step 1: Import types and add buildPayload()**

Read the full file to make precise edits.

Add import at top of `web/src/pages/SetterProblemWorkspace.tsx` (line 2, after existing `api` import):

```typescript
import type { ProblemFormState, TestCase, Collaborator } from '../types/problem-workspace'
```

Consolidate the 25+ useState calls into a single `useState<ProblemFormState>` (replace lines 29-68):

```typescript
// Consolidated form state
const [formState, setFormState] = useState<ProblemFormState>({
  title: '',
  description: '',
  inputFormat: '',
  outputFormat: '',
  hint: '',
  timeLimit: 1000,
  memoryLimit: 262144,
  difficulty: 'easy',
  tags: '',
  sampleCases: [],
  testcases: [],
  checkerType: 'exact',
  floatEpsilon: 1e-6,
  spj: false,
  spjLanguage: 'cpp-gpp-64',
  spjSourceCode: '',
  interactive: false,
  interactorLanguage: 'cpp-gpp-64',
  interactorSourceCode: '',
  visible: true,
})

// Test Cases State (supplementary, outside formState for immediate UI updates)
const [newTestCase, setNewTestCase] = useState<TestCase>({ input_name: '', output_name: '', score: 10 })
const [batchScore, setBatchScore] = useState<number>(10)
const [batchApplying, setBatchApplying] = useState(false)
```

Add `buildPayload()` function after the `useEffect` (after line 105):

```typescript
const buildPayload = (overrides: Partial<ProblemFormState> = {}): Record<string, unknown> => {
  const merged = { ...formState, ...overrides }
  return {
    title: merged.title,
    description: merged.description,
    input_format: merged.inputFormat,
    output_format: merged.outputFormat,
    hint: merged.hint,
    time_limit: Number(merged.timeLimit),
    memory_limit: Number(merged.memoryLimit),
    difficulty: merged.difficulty,
    tags: merged.tags.split(',').map(t => t.trim()).filter(t => t !== ''),
    sample_cases: merged.sampleCases,
    testcase_score: merged.testcases,
    spj: merged.checkerType === 'custom',
    spj_language: merged.spjLanguage,
    spj_source_code: merged.spjSourceCode,
    checker_type: merged.checkerType,
    float_epsilon: merged.floatEpsilon,
    interactive: merged.interactive,
    interactor_language: merged.interactorLanguage,
    interactor_source_code: merged.interactorSourceCode,
    visible: merged.visible,
  }
}

const handleSave = async (overrides?: Partial<ProblemFormState>) => {
  setSaving(true)
  setError(null)
  setSuccess(null)
  try {
    const payload = buildPayload(overrides)
    await api.problems.update(problem.slug, payload)
    setSuccess('Problem saved successfully!')
    loadProblem()
  } catch (err: any) {
    setError(err.message || 'Failed to save problem')
  } finally {
    setSaving(false)
  }
}
```

- [ ] **Step 2: Update loadProblem() to populate formState**

Replace the individual `setTitle`, `setDescription`, etc. calls in `loadProblem()` (lines 74-93) with:

```typescript
const loadProblem = async () => {
  if (!slug) return
  try {
    const data = await api.problems.get(slug)
    setProblem(data)
    setFormState({
      title: data.title || '',
      description: data.description || '',
      inputFormat: data.input_format || '',
      outputFormat: data.output_format || '',
      hint: data.hint || '',
      timeLimit: data.time_limit || 1000,
      memoryLimit: data.memory_limit || 262144,
      difficulty: data.difficulty || 'easy',
      tags: data.tags?.join(', ') || '',
      sampleCases: data.sample_cases || [],
      testcases: data.testcase_score || [],
      checkerType: data.checker_type || 'exact',
      floatEpsilon: data.float_epsilon ?? 1e-6,
      spj: data.spj || false,
      spjLanguage: data.spj_language || 'cpp-gpp-64',
      spjSourceCode: data.spj_source_code || '',
      interactive: data.interactive || false,
      interactorLanguage: data.interactor_language || 'cpp-gpp-64',
      interactorSourceCode: data.interactor_source_code || '',
      visible: data.visible !== false,
    })

    // Load permissions
    const permData = await api.problems.getPermissions(data.slug)
    setCollaborators(permData.data || [])
  } catch (err: any) {
    setError(err.message || 'Failed to load problem workspace')
  }
}
```

- [ ] **Step 3: Replace all 5 handler bodies with buildPayload() + handleSave()**

Replace `handleSaveStatement` (lines 107-144) with:

```typescript
const handleSaveStatement = async (e: React.FormEvent) => {
  e.preventDefault()
  await handleSave()
}
```

Replace `handleAddTestCaseScore` (lines 160-199) with:

```typescript
const handleAddTestCaseScore = async () => {
  if (!newTestCase.input_name || !newTestCase.output_name) {
    setError('Input and Output file names are required')
    return
  }
  setError(null)
  setSuccess(null)
  const updated = [...formState.testcases, newTestCase]
  setFormState(prev => ({ ...prev, testcases: updated }))
  setNewTestCase({ input_name: '', output_name: '', score: 10 })
  try {
    await api.problems.update(problem.slug, buildPayload({ testcases: updated }))
    setSuccess('Testcase scores updated successfully!')
  } catch (err: any) {
    setError(err.message || 'Failed to save testcase scores')
  }
}
```

Replace `handleRemoveTestCaseScore` (lines 201-235) with:

```typescript
const handleRemoveTestCaseScore = async (index: number) => {
  setError(null)
  setSuccess(null)
  const updated = formState.testcases.filter((_, i) => i !== index)
  setFormState(prev => ({ ...prev, testcases: updated }))
  try {
    await api.problems.update(problem.slug, buildPayload({ testcases: updated }))
    setSuccess('Testcase score removed successfully!')
  } catch (err: any) {
    setError(err.message || 'Failed to save testcase scores')
  }
}
```

Replace `handleBatchSetScores` (lines 237-278) with:

```typescript
const handleBatchSetScores = async () => {
  if (formState.testcases.length === 0) {
    setError('No testcases registered to allocate scores')
    return
  }
  setError(null)
  setSuccess(null)
  setBatchApplying(true)
  const updated = formState.testcases.map(tc => ({ ...tc, score: batchScore }))
  setFormState(prev => ({ ...prev, testcases: updated }))
  try {
    await api.problems.update(problem.slug, buildPayload({ testcases: updated }))
    setSuccess(`All ${formState.testcases.length} testcase scores set to ${batchScore} points successfully!`)
  } catch (err: any) {
    setError(err.message || 'Failed to update testcase scores')
  } finally {
    setBatchApplying(false)
  }
}
```

Replace `handleSaveChecker` (lines 280-316) with:

```typescript
const handleSaveChecker = async () => {
  setSaving(true)
  setError(null)
  setSuccess(null)
  try {
    const payload = buildPayload({
      spj: formState.checkerType === 'custom',
    })
    await api.problems.update(problem.slug, payload)
    setSuccess('Checker configuration saved successfully!')
    loadProblem()
  } catch (err: any) {
    setError(err.message || 'Failed to save checker configuration')
  } finally {
    setSaving(false)
  }
}
```

- [ ] **Step 4: Update JSX references from individual state vars to formState.***

Replace all references in the JSX:
- `title` → `formState.title`
- `description` → `formState.description`
- `inputFormat` → `formState.inputFormat`
- `outputFormat` → `formState.outputFormat`
- `hint` → `formState.hint`
- `timeLimit` → `formState.timeLimit`
- `memoryLimit` → `formState.memoryLimit`
- `difficulty` → `formState.difficulty`
- `tags` → `formState.tags`
- `sampleCases` → `formState.sampleCases`
- `testcases` → `formState.testcases`
- `checkerType` → `formState.checkerType`
- `floatEpsilon` → `formState.floatEpsilon`
- `spj` → `formState.spj`
- `spjLanguage` → `formState.spjLanguage`
- `spjSourceCode` → `formState.spjSourceCode`
- `interactive` → `formState.interactive`
- `interactorLanguage` → `formState.interactorLanguage`
- `interactorSourceCode` → `formState.interactorSourceCode`
- `visible` → `formState.visible`

And replace all onChange handlers:
- `e => setTitle(e.target.value)` → `e => setFormState(prev => ({...prev, title: e.target.value}))`
- `e => setDescription(...)` → `e => setFormState(prev => ({...prev, description: ...}))`
- etc.

Use a helper to reduce boilerplate for text inputs:

```typescript
const updateField = <K extends keyof ProblemFormState>(
  key: K,
  value: ProblemFormState[K]
) => {
  setFormState(prev => ({ ...prev, [key]: value }))
}
```

Add this helper after `buildPayload()`.

Then replace onChange handlers like:
- `onChange={e => setFormState(prev => ({...prev, title: e.target.value}))}` 
→ can remain verbose since it's straightforward.

- [ ] **Step 5: Run TypeScript check**

Run: `npx tsc -b --noEmit`
Expected: No compilation errors.

- [ ] **Step 6: Run tests to verify baseline still passes**

Run: `npx vitest run src/pages/SetterProblemWorkspace.test.tsx --reporter=verbose`
Expected: All tests pass (behavior unchanged, just internal state restructuring).

- [ ] **Step 7: Commit**

```bash
git add web/src/pages/SetterProblemWorkspace.tsx
git commit -m "refactor(workspace): consolidate state into ProblemFormState and extract buildPayload()"
```

---

## WAVE 1: Tab Sub-Components

### Task 1.1: StatementTab

**Files:**
- Create: `web/src/components/SetterWorkspace/StatementTab.tsx`

### Scenario Contract
| Given | When | Then |
|-------|------|------|
| Statement tab selected | Component renders | Shows all statement fields: title, description, limits, tags, sample cases |
| User edits a field | onChange fires | Form state updates via callback prop |
| Save clicked | onSave fires | Calls save handler with full payload |

- [ ] **Step 1: Write the test file for StatementTab**

Create `web/src/components/SetterWorkspace/StatementTab.test.tsx`:

```typescript
import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import StatementTab from './StatementTab'
import type { ProblemFormState } from '../../types/problem-workspace'

const baseState: ProblemFormState = {
  title: 'Test Problem',
  description: 'Description here',
  inputFormat: 'Two ints',
  outputFormat: 'Sum',
  hint: 'Think',
  timeLimit: 1000,
  memoryLimit: 262144,
  difficulty: 'easy',
  tags: 'math, ad-hoc',
  sampleCases: [{ input: '1 2', output: '3', explanation: '' }],
  testcases: [],
  checkerType: 'exact',
  floatEpsilon: 1e-6,
  spj: false,
  spjLanguage: 'cpp-gpp-64',
  spjSourceCode: '',
  interactive: false,
  interactorLanguage: 'cpp-gpp-64',
  interactorSourceCode: '',
  visible: true,
}

test('renders all statement form fields', () => {
  const onUpdate = vi.fn()
  render(<StatementTab formState={baseState} onUpdate={onUpdate} onSave={vi.fn()} />)

  expect(screen.getByDisplayValue('Test Problem')).toBeInTheDocument()
  expect(screen.getByDisplayValue('Description here')).toBeInTheDocument()
  expect(screen.getByDisplayValue('Two ints')).toBeInTheDocument()
  expect(screen.getByDisplayValue('Sum')).toBeInTheDocument()
  expect(screen.getByDisplayValue('1000')).toBeInTheDocument()
  expect(screen.getByDisplayValue('262144')).toBeInTheDocument()
  expect(screen.getByDisplayValue('math, ad-hoc')).toBeInTheDocument()
})

test('calls onUpdate when title changes', () => {
  const onUpdate = vi.fn()
  render(<StatementTab formState={baseState} onUpdate={onUpdate} onSave={vi.fn()} />)

  const titleInput = screen.getByDisplayValue('Test Problem')
  fireEvent.change(titleInput, { target: { value: 'New Title' } })

  expect(onUpdate).toHaveBeenCalledWith('title', 'New Title')
})

test('calls onSave when save button clicked', () => {
  const onSave = vi.fn()
  render(<StatementTab formState={baseState} onUpdate={vi.fn()} onSave={onSave} />)

  fireEvent.click(screen.getByText('Save Statement'))
  expect(onSave).toHaveBeenCalledTimes(1)
})

test('adds a new sample case', () => {
  const onUpdate = vi.fn()
  render(<StatementTab formState={baseState} onUpdate={onUpdate} onSave={vi.fn()} />)

  fireEvent.click(screen.getByText('+ Add Sample'))
  expect(onUpdate).toHaveBeenCalledWith(
    'sampleCases',
    [...baseState.sampleCases, { input: '', output: '', explanation: '' }]
  )
})

test('removes a sample case', () => {
  const onUpdate = vi.fn()
  render(<StatementTab formState={baseState} onUpdate={onUpdate} onSave={vi.fn()} />)

  const removeButton = screen.getByText('Remove')
  fireEvent.click(removeButton)

  expect(onUpdate).toHaveBeenCalledWith('sampleCases', [])
})
```

- [ ] **Step 2: Run test to verify it fails (RED)**

Run: `npx vitest run src/components/SetterWorkspace/StatementTab.test.tsx --reporter=verbose`
Expected: FAIL — "Cannot find module './StatementTab'"

- [ ] **Step 3: Create StatementTab component**

Write `web/src/components/SetterWorkspace/StatementTab.tsx`:

```typescript
import type { ProblemFormState } from '../../types/problem-workspace'

interface StatementTabProps {
  formState: ProblemFormState
  saving: boolean
  onUpdate: <K extends keyof ProblemFormState>(key: K, value: ProblemFormState[K]) => void
  onSave: (e: React.FormEvent) => void
}

export default function StatementTab({ formState, saving, onUpdate, onSave }: StatementTabProps) {
  return (
    <form onSubmit={onSave} className="space-y-4">
      <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Edit Problem Statement</h2>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Title</label>
          <input
            type="text"
            value={formState.title}
            onChange={e => onUpdate('title', e.target.value)}
            className="w-full border rounded px-3 py-1.5 text-sm"
            required
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Tags (comma-separated)</label>
          <input
            type="text"
            value={formState.tags}
            onChange={e => onUpdate('tags', e.target.value)}
            className="w-full border rounded px-3 py-1.5 text-sm"
          />
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Difficulty</label>
          <select
            value={formState.difficulty}
            onChange={e => onUpdate('difficulty', e.target.value)}
            className="w-full border rounded px-3 py-1.5 text-sm"
          >
            <option value="easy">Easy</option>
            <option value="medium">Medium</option>
            <option value="hard">Hard</option>
          </select>
        </div>
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Time Limit (ms)</label>
          <input
            type="number"
            value={formState.timeLimit}
            onChange={e => onUpdate('timeLimit', Number(e.target.value))}
            className="w-full border rounded px-3 py-1.5 text-sm"
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Memory Limit (KB)</label>
          <input
            type="number"
            value={formState.memoryLimit}
            onChange={e => onUpdate('memoryLimit', Number(e.target.value))}
            className="w-full border rounded px-3 py-1.5 text-sm"
          />
        </div>
      </div>

      <div>
        <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Description (Markdown Supported)</label>
        <textarea
          value={formState.description}
          onChange={e => onUpdate('description', e.target.value)}
          rows={8}
          className="w-full border rounded px-3 py-1.5 text-sm font-mono"
          required
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Input Format</label>
          <textarea
            value={formState.inputFormat}
            onChange={e => onUpdate('inputFormat', e.target.value)}
            rows={4}
            className="w-full border rounded px-3 py-1.5 text-sm"
          />
        </div>
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Output Format</label>
          <textarea
            value={formState.outputFormat}
            onChange={e => onUpdate('outputFormat', e.target.value)}
            rows={4}
            className="w-full border rounded px-3 py-1.5 text-sm"
          />
        </div>
      </div>

      <div>
        <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Hint</label>
        <textarea
          value={formState.hint}
          onChange={e => onUpdate('hint', e.target.value)}
          rows={2}
          className="w-full border rounded px-3 py-1.5 text-sm"
        />
      </div>

      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold text-sm text-gray-700">Sample Cases</h3>
          <button
            type="button"
            onClick={() => onUpdate('sampleCases', [...formState.sampleCases, { input: '', output: '', explanation: '' }])}
            className="bg-blue-600 text-white px-3 py-1.5 rounded text-xs hover:bg-blue-700 transition-colors cursor-pointer"
          >
            + Add Sample
          </button>
        </div>

        {formState.sampleCases.map((sc, i) => (
          <div key={i} className="border border-gray-200 rounded-lg p-4 space-y-3">
            <div className="flex items-center justify-between">
              <span className="font-medium text-sm">Sample {i + 1}</span>
              <button
                type="button"
                onClick={() => onUpdate('sampleCases', formState.sampleCases.filter((_, j) => j !== i))}
                className="text-red-600 hover:text-red-800 text-xs cursor-pointer"
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
                    const updated = [...formState.sampleCases]
                    updated[i] = { ...updated[i], input: e.target.value }
                    onUpdate('sampleCases', updated)
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
                    const updated = [...formState.sampleCases]
                    updated[i] = { ...updated[i], output: e.target.value }
                    onUpdate('sampleCases', updated)
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
                  const updated = [...formState.sampleCases]
                  updated[i] = { ...updated[i], explanation: e.target.value }
                  onUpdate('sampleCases', updated)
                }}
                className="w-full text-sm border border-gray-300 rounded px-2 py-1.5"
              />
            </div>
          </div>
        ))}
      </div>

      <button
        type="submit"
        disabled={saving}
        className="bg-blue-600 text-white px-5 py-2 rounded text-sm font-semibold hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer"
      >
        {saving ? 'Saving...' : 'Save Statement'}
      </button>
    </form>
  )
}
```

- [ ] **Step 4: Run test to verify it passes (GREEN)**

Run: `npx vitest run src/components/SetterWorkspace/StatementTab.test.tsx --reporter=verbose`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/SetterWorkspace/StatementTab.tsx web/src/components/SetterWorkspace/StatementTab.test.tsx
git commit -m "feat(workspace): extract StatementTab sub-component"
```

---

### Task 1.2: TestCasesTab

**Files:**
- Create: `web/src/components/SetterWorkspace/TestCasesTab.tsx`

### Scenario Contract
| Given | When | Then |
|-------|------|------|
| No testcases registered | Tab renders | Shows "No testcase scores registered" empty state |
| Testcases exist | Tab renders | Shows table with input/output/score columns |
| User clicks "Apply to All" | Batch set triggered | All scores updated to batch value |
| User clicks "Add File Match" | New testcase added | Calls onAdd with current newTestCase values |

- [ ] **Step 1: Write the test file**

Create `web/src/components/SetterWorkspace/TestCasesTab.test.tsx`:

```typescript
import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import TestCasesTab from './TestCasesTab'
import type { TestCase } from '../../types/problem-workspace'

const testcases: TestCase[] = [
  { input_name: '01.in', output_name: '01.out', score: 10 },
  { input_name: '02.in', output_name: '02.out', score: 20 },
]

test('shows empty state when no testcases', () => {
  render(
    <TestCasesTab
      testcases={[]}
      newTestCase={{ input_name: '', output_name: '', score: 10 }}
      batchScore={10}
      batchApplying={false}
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onBatchSet={vi.fn()}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  expect(screen.getByText(/No testcase scores registered/)).toBeInTheDocument()
})

test('renders testcase table with scores', () => {
  render(
    <TestCasesTab
      testcases={testcases}
      newTestCase={{ input_name: '', output_name: '', score: 10 }}
      batchScore={10}
      batchApplying={false}
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onBatchSet={vi.fn()}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  expect(screen.getByText('01.in')).toBeInTheDocument()
  expect(screen.getByText('02.out')).toBeInTheDocument()
  expect(screen.getByText('20 pts')).toBeInTheDocument()
})

test('shows total testcase count and points', () => {
  render(
    <TestCasesTab
      testcases={testcases}
      newTestCase={{ input_name: '', output_name: '', score: 10 }}
      batchScore={10}
      batchApplying={false}
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onBatchSet={vi.fn()}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  expect(screen.getByText(/Total Testcases: 2/)).toBeInTheDocument()
  expect(screen.getByText(/Total Points: 30/)).toBeInTheDocument()
})

test('calls onBatchSet when Apply to All clicked', () => {
  const onBatchSet = vi.fn()
  render(
    <TestCasesTab
      testcases={testcases}
      newTestCase={{ input_name: '', output_name: '', score: 10 }}
      batchScore={10}
      batchApplying={false}
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onBatchSet={onBatchSet}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  fireEvent.click(screen.getByText('Apply to All'))
  expect(onBatchSet).toHaveBeenCalledTimes(1)
})

test('calls onAdd when Add File Match clicked', () => {
  const onAdd = vi.fn()
  render(
    <TestCasesTab
      testcases={testcases}
      newTestCase={{ input_name: '03.in', output_name: '03.out', score: 30 }}
      batchScore={10}
      batchApplying={false}
      onAdd={onAdd}
      onRemove={vi.fn()}
      onBatchSet={vi.fn()}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  fireEvent.click(screen.getByText('Add File Match'))
  expect(onAdd).toHaveBeenCalledTimes(1)
})

test('calls onRemove when Remove button clicked', () => {
  const onRemove = vi.fn()
  render(
    <TestCasesTab
      testcases={testcases}
      newTestCase={{ input_name: '', output_name: '', score: 10 }}
      batchScore={10}
      batchApplying={false}
      onAdd={vi.fn()}
      onRemove={onRemove}
      onBatchSet={vi.fn()}
      onUpload={vi.fn()}
      onNewTestCaseChange={vi.fn()}
      onBatchScoreChange={vi.fn()}
    />
  )
  const removeButtons = screen.getAllByText('Remove')
  fireEvent.click(removeButtons[0])
  expect(onRemove).toHaveBeenCalledWith(0)
})
```

- [ ] **Step 2: Run test to verify it fails (RED)**

Run: `npx vitest run src/components/SetterWorkspace/TestCasesTab.test.tsx --reporter=verbose`
Expected: FAIL

- [ ] **Step 3: Create TestCasesTab component**

Write `web/src/components/SetterWorkspace/TestCasesTab.tsx`:

```typescript
import { useRef } from 'react'
import type { TestCase } from '../../types/problem-workspace'

interface TestCasesTabProps {
  testcases: TestCase[]
  newTestCase: TestCase
  batchScore: number
  batchApplying: boolean
  onAdd: () => void
  onRemove: (index: number) => void
  onBatchSet: () => void
  onUpload: (e: React.ChangeEvent<HTMLInputElement>) => void
  onNewTestCaseChange: (tc: TestCase) => void
  onBatchScoreChange: (score: number) => void
}

export default function TestCasesTab({
  testcases,
  newTestCase,
  batchScore,
  batchApplying,
  onAdd,
  onRemove,
  onBatchSet,
  onUpload,
  onNewTestCaseChange,
  onBatchScoreChange,
}: TestCasesTabProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Upload Testcase Package (ZIP)</h2>
        <p className="text-xs text-gray-400 mb-3">
          Upload a ZIP package containing input and output testcases (e.g. `01.in` and `01.out`).
          Files will be parsed and loaded into the problem storage directory on the sandbox filesystem.
        </p>
        <div className="flex gap-3 items-center">
          <input
            type="file"
            ref={fileInputRef}
            onChange={onUpload}
            accept=".zip"
            className="hidden"
          />
          <button
            onClick={() => fileInputRef.current?.click()}
            className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-blue-700 transition-colors cursor-pointer"
          >
            Choose ZIP file
          </button>
        </div>
      </div>

      <hr className="border-gray-200" />

      <div>
        <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Testcase Scores & Breakdown</h2>
        <div className="border border-gray-200 rounded-lg overflow-hidden mb-4">
          <table className="w-full text-sm text-left">
            <thead className="bg-gray-50 text-gray-500 text-xs uppercase font-semibold">
              <tr>
                <th className="px-4 py-2.5">Input File</th>
                <th className="px-4 py-2.5">Output File</th>
                <th className="px-4 py-2.5">Score</th>
                <th className="px-4 py-2.5 text-right">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {testcases.map((tc, idx) => (
                <tr key={idx}>
                  <td className="px-4 py-2.5 font-mono text-xs">{tc.input_name}</td>
                  <td className="px-4 py-2.5 font-mono text-xs">{tc.output_name}</td>
                  <td className="px-4 py-2.5">{tc.score} pts</td>
                  <td className="px-4 py-2.5 text-right">
                    <button
                      onClick={() => onRemove(idx)}
                      className="text-red-600 hover:underline cursor-pointer"
                    >
                      Remove
                    </button>
                  </td>
                </tr>
              ))}
              {testcases.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-4 py-6 text-center text-gray-400 text-xs">
                    No testcase scores registered. Add testcase scores below to match files.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>

        {testcases.length > 0 && (
          <div className="bg-gray-50 rounded-lg p-4 border border-gray-200 mb-4 flex items-end justify-between">
            <div className="flex gap-4 items-end">
              <div>
                <label className="block text-xs text-gray-500 mb-1">Set Score for All Test Cases</label>
                <input
                  type="number"
                  value={batchScore}
                  onChange={e => onBatchScoreChange(Number(e.target.value))}
                  className="border rounded px-3 py-1 text-xs w-28"
                />
              </div>
              <button
                onClick={onBatchSet}
                disabled={batchApplying}
                className="bg-blue-600 text-white px-3 py-1.5 rounded text-xs hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer"
              >
                {batchApplying ? 'Applying...' : 'Apply to All'}
              </button>
            </div>
            <span className="text-xs text-gray-400 font-medium">
              Total Testcases: {testcases.length} | Total Points: {testcases.reduce((sum, tc) => sum + (tc.score || 0), 0)} pts
            </span>
          </div>
        )}

        <div className="bg-gray-50 rounded-lg p-4 border border-gray-200 space-y-4">
          <h4 className="font-semibold text-xs text-gray-500 uppercase tracking-wider">Register TestCase File Matches</h4>
          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="block text-xs text-gray-500 mb-1">Input Name</label>
              <input
                type="text"
                placeholder="e.g. 01.in"
                value={newTestCase.input_name}
                onChange={e => onNewTestCaseChange({ ...newTestCase, input_name: e.target.value })}
                className="w-full border rounded px-3 py-1.5 text-xs font-mono"
              />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Output Name</label>
              <input
                type="text"
                placeholder="e.g. 01.out"
                value={newTestCase.output_name}
                onChange={e => onNewTestCaseChange({ ...newTestCase, output_name: e.target.value })}
                className="w-full border rounded px-3 py-1.5 text-xs font-mono"
              />
            </div>
            <div>
              <label className="block text-xs text-gray-500 mb-1">Score</label>
              <input
                type="number"
                value={newTestCase.score}
                onChange={e => onNewTestCaseChange({ ...newTestCase, score: Number(e.target.value) })}
                className="w-full border rounded px-3 py-1.5 text-xs"
              />
            </div>
          </div>
          <button
            onClick={onAdd}
            className="bg-gray-800 text-white px-3 py-1.5 rounded text-xs hover:bg-black transition-colors cursor-pointer"
          >
            Add File Match
          </button>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Run test to verify it passes (GREEN)**

Run: `npx vitest run src/components/SetterWorkspace/TestCasesTab.test.tsx --reporter=verbose`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/SetterWorkspace/TestCasesTab.tsx web/src/components/SetterWorkspace/TestCasesTab.test.tsx
git commit -m "feat(workspace): extract TestCasesTab sub-component"
```

---

### Task 1.3: CheckerTab

**Files:**
- Create: `web/src/components/SetterWorkspace/CheckerTab.tsx`

### Scenario Contract
| Given | When | Then |
|-------|------|------|
| Checker tab selected | Component renders | Shows checker type selector and SPJ/interactive sections |
| User picks float type | Epsilon input appears | Epsilon field visible for float/float_absolute/float_relative |
| User picks custom | SPJ editor appears | Shows CodeEditor with language selector and presets |
| User toggles interactive | Interactor section appears | Shows interactor language and code editor |

- [ ] **Step 1: Write the test file**

Create `web/src/components/SetterWorkspace/CheckerTab.test.tsx`:

```typescript
import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import CheckerTab from './CheckerTab'
import type { ProblemFormState } from '../../types/problem-workspace'

const baseState: ProblemFormState = {
  title: '', description: '', inputFormat: '', outputFormat: '', hint: '',
  timeLimit: 1000, memoryLimit: 262144, difficulty: 'easy', tags: '',
  sampleCases: [], testcases: [],
  checkerType: 'exact',
  floatEpsilon: 1e-6,
  spj: false,
  spjLanguage: 'cpp-gpp-64',
  spjSourceCode: '',
  interactive: false,
  interactorLanguage: 'cpp-gpp-64',
  interactorSourceCode: '',
  visible: true,
}

test('renders checker type selector with all options', () => {
  render(<CheckerTab formState={baseState} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByText('Checker & Special Judge Configuration')).toBeInTheDocument()
  expect(screen.getByDisplayValue('exact')).toBeInTheDocument()
})

test('shows epsilon input for float checker types', () => {
  const floatState = { ...baseState, checkerType: 'float_absolute' }
  const { rerender } = render(<CheckerTab formState={floatState} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByText('Float Epsilon (Precision Tolerance)')).toBeInTheDocument()

  rerender(<CheckerTab formState={{ ...baseState, checkerType: 'float_relative' }} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByText('Float Epsilon (Precision Tolerance)')).toBeInTheDocument()

  rerender(<CheckerTab formState={{ ...baseState, checkerType: 'exact' }} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.queryByText('Float Epsilon (Precision Tolerance)')).not.toBeInTheDocument()
})

test('shows SPJ editor when checker type is custom', () => {
  const customState = { ...baseState, checkerType: 'custom' }
  render(<CheckerTab formState={customState} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByText('Special Judge (SPJ) Sandbox Environment Protocol')).toBeInTheDocument()
  expect(screen.getByText('Advanced SPJ Presets')).toBeInTheDocument()
})

test('shows interactive section when toggled', () => {
  const interactiveState = { ...baseState, interactive: true }
  render(<CheckerTab formState={interactiveState} onUpdate={vi.fn()} onSave={vi.fn()} saving={false} />)
  expect(screen.getByText('Interactive Problem (Judge communicates via stdin/stdout)')).toBeInTheDocument()
  expect(screen.getByText('Interactor Language')).toBeInTheDocument()
  expect(screen.getByText('Interactor Source Code')).toBeInTheDocument()
})

test('calls onSave when save button clicked', () => {
  const onSave = vi.fn()
  render(<CheckerTab formState={baseState} onUpdate={vi.fn()} onSave={onSave} saving={false} />)
  fireEvent.click(screen.getByText('Save Checker Configuration'))
  expect(onSave).toHaveBeenCalledTimes(1)
})
```

- [ ] **Step 2: Run test to verify it fails (RED)**

Run: `npx vitest run src/components/SetterWorkspace/CheckerTab.test.tsx --reporter=verbose`
Expected: FAIL

- [ ] **Step 3: Create CheckerTab component**

Write `web/src/components/SetterWorkspace/CheckerTab.tsx`:

```typescript
import type { ProblemFormState } from '../../types/problem-workspace'
import CodeEditor from '../CodeEditor'

interface CheckerTabProps {
  formState: ProblemFormState
  saving: boolean
  onUpdate: <K extends keyof ProblemFormState>(key: K, value: ProblemFormState[K]) => void
  onSave: () => void
}

export default function CheckerTab({ formState, saving, onUpdate, onSave }: CheckerTabProps) {
  const precisionFloatPreset = `#include <iostream>
#include <fstream>
#include <cmath>

using namespace std;

int main(int argc, char* argv[]) {
    if (argc < 4) {
        cerr << "Usage: spj <input> <user> <answer>" << endl;
        return 2;
    }
    
    ifstream fin(argv[1]);
    ifstream fuser(argv[2]);
    ifstream fans(argv[3]);
    
    double userVal, ansVal;
    if (!(fuser >> userVal)) {
        cerr << "Wrong Answer: Failed to read user float token" << endl;
        return 1;
    }
    if (!(fans >> ansVal)) {
        cerr << "System Error: Failed to read expected answer float token" << endl;
        return 2;
    }
    
    double diff = abs(userVal - ansVal);
    if (diff > 1e-9 && diff / max(1.0, abs(ansVal)) > 1e-9) {
        cerr << "Wrong Answer: Difference too large! Expected " << ansVal << ", got " << userVal << " (diff: " << diff << ")" << endl;
        return 1;
    }
    
    cout << "OK: Floats match within 1e-9" << endl;
    return 0;
}`

  const arrayGraphPreset = `#include <iostream>
#include <fstream>
#include <vector>

using namespace std;

int main(int argc, char* argv[]) {
    ifstream fin(argv[1]);   // Input case
    ifstream fuser(argv[2]); // User stdout
    ifstream fans(argv[3]);  // Expected output
    
    int n;
    fin >> n;
    
    vector<int> userArr(n);
    for (int i = 0; i < n; i++) {
        if (!(fuser >> userArr[i])) {
            cerr << "Wrong Answer: Insufficient numbers of tokens" << endl;
            return 1;
        }
    }
    
    for (int i = 1; i < n; i++) {
        if (userArr[i] < userArr[i-1]) {
            cerr << "Wrong Answer: Array is not sorted at index " << i << endl;
            return 1;
        }
    }
    
    cout << "OK: Sorted output verified" << endl;
    return 0;
}`

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Checker & Special Judge Configuration</h2>

      <div>
        <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Checker Method</label>
        <select
          value={formState.checkerType}
          onChange={e => onUpdate('checkerType', e.target.value)}
          className="border rounded px-3 py-1.5 text-sm"
        >
          <option value="exact">Exact Bytes Match (Standard)</option>
          <option value="lines">Lines Differences (Ignore Trailing Spaces)</option>
          <option value="float">Float Tolerance Precision</option>
          <option value="float_absolute">Floating Point (Absolute Epsilon)</option>
          <option value="float_relative">Floating Point (Relative Epsilon)</option>
          <option value="custom">Custom Special Judge (SPJ)</option>
        </select>
      </div>

      {(formState.checkerType === 'float' || formState.checkerType === 'float_absolute' || formState.checkerType === 'float_relative') && (
        <div>
          <label className="block text-xs font-semibold text-gray-500 uppercase mb-1">Float Epsilon (Precision Tolerance)</label>
          <input
            type="number"
            step="any"
            value={formState.floatEpsilon}
            onChange={e => onUpdate('floatEpsilon', Number(e.target.value))}
            className="border rounded px-3 py-1.5 text-sm w-48 font-mono"
          />
        </div>
      )}

      {formState.checkerType === 'custom' && (
        <div className="space-y-4">
          <div className="bg-gray-900 border border-gray-800 rounded-lg p-5 text-gray-300 shadow-md space-y-3">
            <div className="flex items-center space-x-2 text-yellow-400">
              <span className="w-2.5 h-2.5 rounded-full bg-yellow-400 animate-pulse" />
              <p className="font-bold text-xs uppercase tracking-wider">Special Judge (SPJ) Sandbox Environment Protocol</p>
            </div>
            <p className="text-xs leading-relaxed text-gray-400">
              Your compiled C++ checker binary runs inside a secure isolated sandbox with execution parameters:
            </p>
            <div className="bg-black/40 border border-gray-800 rounded p-2.5 font-mono text-[11px] text-emerald-400 select-all">
              ./spj input.txt user.txt answer.txt
            </div>
            <div className="grid grid-cols-2 gap-4 pt-2 text-[11px]">
              <div className="space-y-1.5">
                <span className="font-semibold block text-gray-400 uppercase tracking-wider text-[10px]">Compilation Outputs</span>
                <ul className="list-disc pl-4 space-y-0.5 text-gray-500">
                  <li>Standard streams are fully captured</li>
                  <li>Errors output on <span className="font-semibold text-gray-400">stderr</span> as details</li>
                </ul>
              </div>
              <div className="space-y-1.5">
                <span className="font-semibold block text-gray-400 uppercase tracking-wider text-[10px]">Return Status Codes</span>
                <ul className="list-disc pl-4 space-y-0.5 text-gray-500">
                  <li><span className="font-semibold text-emerald-400">exit status 0</span>: Accepted (AC)</li>
                  <li><span className="font-semibold text-rose-400">exit status 1</span>: Wrong Answer (WA)</li>
                </ul>
              </div>
            </div>
          </div>

          <div className="bg-white border rounded-lg p-4 shadow-sm space-y-4">
            <div className="flex justify-between items-center border-b pb-3">
              <div>
                <span className="text-xs text-gray-600 font-bold block uppercase tracking-wider">Advanced SPJ Presets</span>
                <span className="text-[11px] text-gray-400">Populate boilerplate templates directly into the editor</span>
              </div>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => onUpdate('spjSourceCode', precisionFloatPreset)}
                  className="bg-gray-50 border border-gray-200 text-gray-700 px-3 py-1.5 rounded-md text-xs hover:bg-gray-100 hover:text-black transition-colors font-semibold cursor-pointer"
                >
                  Precision Float Presets
                </button>
                <button
                  type="button"
                  onClick={() => onUpdate('spjSourceCode', arrayGraphPreset)}
                  className="bg-gray-50 border border-gray-200 text-gray-700 px-3 py-1.5 rounded-md text-xs hover:bg-gray-100 hover:text-black transition-colors font-semibold cursor-pointer"
                >
                  Array/Graph Presets
                </button>
              </div>
            </div>

            <div>
              <label className="block text-[10px] font-bold text-gray-400 uppercase tracking-wider mb-1">SPJ Sandbox Language</label>
              <select
                value={formState.spjLanguage}
                onChange={e => onUpdate('spjLanguage', e.target.value)}
                className="border rounded px-3 py-1.5 text-sm"
              >
                <option value="cpp-gpp-64">C++ (G++ 64-bit)</option>
              </select>
            </div>

            <div>
              <label className="block text-[10px] font-bold text-gray-400 uppercase tracking-wider mb-1">SPJ Source Code Editor</label>
              <div className="border rounded overflow-hidden shadow-inner bg-gray-50">
                <CodeEditor
                  language={formState.spjLanguage}
                  value={formState.spjSourceCode}
                  onChange={(val: string) => onUpdate('spjSourceCode', val)}
                  height="350px"
                />
              </div>
            </div>
          </div>
        </div>
      )}

      <hr className="border-gray-200" />

      <div className="space-y-3">
        <h3 className="text-sm font-bold text-gray-700 uppercase tracking-wider">Interactive Problem</h3>
        <div className="flex items-center gap-2">
          <input
            type="checkbox"
            id="interactive"
            checked={formState.interactive}
            onChange={e => onUpdate('interactive', e.target.checked)}
            className="rounded"
          />
          <label htmlFor="interactive" className="text-sm font-medium text-gray-700">
            Interactive Problem (Judge communicates via stdin/stdout)
          </label>
        </div>

        {formState.interactive && (
          <div className="space-y-3 pl-6 border-l-2 border-blue-200">
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Interactor Language</label>
              <select
                value={formState.interactorLanguage}
                onChange={e => onUpdate('interactorLanguage', e.target.value)}
                className="border border-gray-300 rounded px-2 py-1.5 text-sm"
              >
                <option value="cpp-gpp-64">C++ (G++ 64-bit)</option>
                <option value="python">Python 3</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Interactor Source Code</label>
              <div className="border rounded overflow-hidden shadow-inner bg-gray-50">
                <CodeEditor
                  language={formState.interactorLanguage}
                  value={formState.interactorSourceCode}
                  onChange={(val: string) => onUpdate('interactorSourceCode', val)}
                  height="200px"
                />
              </div>
            </div>
          </div>
        )}
      </div>

      <button
        onClick={onSave}
        disabled={saving}
        className="bg-blue-600 text-white px-5 py-2 rounded text-sm font-semibold hover:bg-blue-700 disabled:opacity-50 transition-colors cursor-pointer"
      >
        {saving ? 'Saving...' : 'Save Checker Configuration'}
      </button>
    </div>
  )
}
```

- [ ] **Step 4: Run test to verify it passes (GREEN)**

Run: `npx vitest run src/components/SetterWorkspace/CheckerTab.test.tsx --reporter=verbose`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/SetterWorkspace/CheckerTab.tsx web/src/components/SetterWorkspace/CheckerTab.test.tsx
git commit -m "feat(workspace): extract CheckerTab sub-component"
```

---

### Task 1.4: PermissionsTab

**Files:**
- Create: `web/src/components/SetterWorkspace/PermissionsTab.tsx`

### Scenario Contract
| Given | When | Then |
|-------|------|------|
| Collaborators exist | Tab renders | Shows table with username, access level, and action buttons |
| Owner collaborator | Row renders | Shows "Primary Owner" badge, no Remove button |
| Non-owner collaborator | Row renders | Shows Remove button that triggers callback |
| User fills add form | Clicks "Share Permissions" | Calls onAdd with username and access level |

- [ ] **Step 1: Write the test file**

Create `web/src/components/SetterWorkspace/PermissionsTab.test.tsx`:

```typescript
import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import PermissionsTab from './PermissionsTab'
import type { Collaborator } from '../../types/problem-workspace'

const collaborators: Collaborator[] = [
  { problem_id: 'p1', user_id: 'u1', username: 'owner', access_level: 'owner' },
  { problem_id: 'p1', user_id: 'u2', username: 'coauthor1', access_level: 'co_author' },
  { problem_id: 'p1', user_id: 'u3', username: 'tester1', access_level: 'tester' },
]

test('renders collaborators table', () => {
  render(
    <PermissionsTab
      collaborators={collaborators}
      newUsername=""
      newAccessLevel="co_author"
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  expect(screen.getByText('owner')).toBeInTheDocument()
  expect(screen.getByText('coauthor1')).toBeInTheDocument()
  expect(screen.getByText('tester1')).toBeInTheDocument()
})

test('shows Primary Owner badge for owner', () => {
  render(
    <PermissionsTab
      collaborators={collaborators}
      newUsername=""
      newAccessLevel="co_author"
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  expect(screen.getByText('Primary Owner')).toBeInTheDocument()
})

test('shows correct access level badges', () => {
  render(
    <PermissionsTab
      collaborators={collaborators}
      newUsername=""
      newAccessLevel="co_author"
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  expect(screen.getByText('owner').closest('span')).toHaveClass('bg-purple-100')
  expect(screen.getByText('co_author').closest('span')).toHaveClass('bg-blue-100')
  expect(screen.getByText('tester').closest('span')).toHaveClass('bg-gray-100')
})

test('does not show Remove button for owner', () => {
  render(
    <PermissionsTab
      collaborators={collaborators}
      newUsername=""
      newAccessLevel="co_author"
      onAdd={vi.fn()}
      onRemove={vi.fn()}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  // Only 2 Remove buttons (for non-owners)
  const removeButtons = screen.getAllByText('Remove')
  expect(removeButtons).toHaveLength(2)
})

test('calls onRemove when Remove clicked', () => {
  const onRemove = vi.fn()
  render(
    <PermissionsTab
      collaborators={collaborators}
      newUsername=""
      newAccessLevel="co_author"
      onAdd={vi.fn()}
      onRemove={onRemove}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  const removeButtons = screen.getAllByText('Remove')
  fireEvent.click(removeButtons[0])
  expect(onRemove).toHaveBeenCalledWith('u2')
})

test('calls onAdd when Share Permissions clicked', () => {
  const onAdd = vi.fn()
  render(
    <PermissionsTab
      collaborators={[]}
      newUsername="newuser"
      newAccessLevel="tester"
      onAdd={onAdd}
      onRemove={vi.fn()}
      onNewUsernameChange={vi.fn()}
      onNewAccessLevelChange={vi.fn()}
    />
  )
  fireEvent.click(screen.getByText('Share Permissions'))
  expect(onAdd).toHaveBeenCalledTimes(1)
})
```

- [ ] **Step 2: Run test to verify it fails (RED)**

Run: `npx vitest run src/components/SetterWorkspace/PermissionsTab.test.tsx --reporter=verbose`
Expected: FAIL

- [ ] **Step 3: Create PermissionsTab component**

Write `web/src/components/SetterWorkspace/PermissionsTab.tsx`:

```typescript
import type { Collaborator } from '../../types/problem-workspace'

interface PermissionsTabProps {
  collaborators: Collaborator[]
  newUsername: string
  newAccessLevel: string
  onAdd: () => void
  onRemove: (userId: string) => void
  onNewUsernameChange: (username: string) => void
  onNewAccessLevelChange: (level: string) => void
}

export default function PermissionsTab({
  collaborators,
  newUsername,
  newAccessLevel,
  onAdd,
  onRemove,
  onNewUsernameChange,
  onNewAccessLevelChange,
}: PermissionsTabProps) {
  return (
    <div className="space-y-6">
      <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Problem Collaborators</h2>

      <div className="border border-gray-200 rounded-lg overflow-hidden mb-4">
        <table className="w-full text-sm text-left">
          <thead className="bg-gray-50 text-gray-500 text-xs uppercase font-semibold">
            <tr>
              <th className="px-4 py-2.5">User</th>
              <th className="px-4 py-2.5">Access Level</th>
              <th className="px-4 py-2.5 text-right">Action</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {collaborators.map((c, idx) => (
              <tr key={idx}>
                <td className="px-4 py-2.5 font-medium">{c.username}</td>
                <td className="px-4 py-2.5">
                  <span className={`px-2 py-0.5 rounded text-xs font-semibold uppercase tracking-wider ${
                    c.access_level === 'owner' ? 'bg-purple-100 text-purple-800' :
                    c.access_level === 'co_author' ? 'bg-blue-100 text-blue-800' : 'bg-gray-100 text-gray-800'
                  }`}>
                    {c.access_level}
                  </span>
                </td>
                <td className="px-4 py-2.5 text-right">
                  {c.access_level !== 'owner' ? (
                    <button
                      onClick={() => onRemove(c.user_id)}
                      className="text-red-600 hover:underline cursor-pointer"
                    >
                      Remove
                    </button>
                  ) : (
                    <span className="text-gray-400 text-xs">Primary Owner</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-4">
        <h4 className="font-semibold text-xs text-gray-500 uppercase tracking-wider">Add Collaborator</h4>
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-xs text-gray-500 mb-1">Username</label>
            <input
              type="text"
              placeholder="Enter username"
              value={newUsername}
              onChange={e => onNewUsernameChange(e.target.value)}
              className="w-full border rounded px-3 py-1.5 text-xs"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-500 mb-1">Access Level</label>
            <select
              value={newAccessLevel}
              onChange={e => onNewAccessLevelChange(e.target.value)}
              className="w-full border rounded px-3 py-1.5 text-xs bg-white"
            >
              <option value="co_author">Co-Author (Full Edit Permissions)</option>
              <option value="tester">Tester (Read & Submit Private Problem)</option>
            </select>
          </div>
        </div>
        <button
          onClick={onAdd}
          className="bg-gray-800 text-white px-3 py-1.5 rounded text-xs hover:bg-black transition-colors cursor-pointer"
        >
          Share Permissions
        </button>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Run test to verify it passes (GREEN)**

Run: `npx vitest run src/components/SetterWorkspace/PermissionsTab.test.tsx --reporter=verbose`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/SetterWorkspace/PermissionsTab.tsx web/src/components/SetterWorkspace/PermissionsTab.test.tsx
git commit -m "feat(workspace): extract PermissionsTab sub-component"
```

---

### Task 1.5: SettingsTab

**Files:**
- Create: `web/src/components/SetterWorkspace/SettingsTab.tsx`

### Scenario Contract
| Given | When | Then |
|-------|------|------|
| Settings tab selected | Component renders | Shows visibility toggle and danger zone |
| User toggles visibility | onChange fires | Visible state toggled via callback |
| User clicks "Update Visibility" | onSave fires | Calls save handler |
| User clicks "Delete Problem" | onDelete fires with confirmation | Calls onDelete |

- [ ] **Step 1: Write the test file**

Create `web/src/components/SetterWorkspace/SettingsTab.test.tsx`:

```typescript
import { render, screen, fireEvent } from '@testing-library/react'
import { expect, test, vi } from 'vitest'
import SettingsTab from './SettingsTab'

test('renders visibility toggle and danger zone', () => {
  render(
    <SettingsTab
      visible={true}
      saving={false}
      onUpdateVisibility={vi.fn()}
      onToggleVisibility={vi.fn()}
      onDelete={vi.fn()}
    />
  )
  expect(screen.getByText('Workspace Settings')).toBeInTheDocument()
  expect(screen.getByText('Problem Visibility')).toBeInTheDocument()
  expect(screen.getByText('Danger Zone')).toBeInTheDocument()
  expect(screen.getByText('Delete Problem')).toBeInTheDocument()
})

test('calls onUpdateVisibility when Update Visibility clicked', () => {
  const onUpdateVisibility = vi.fn()
  render(
    <SettingsTab
      visible={true}
      saving={false}
      onUpdateVisibility={onUpdateVisibility}
      onToggleVisibility={vi.fn()}
      onDelete={vi.fn()}
    />
  )
  fireEvent.click(screen.getByText('Update Visibility'))
  expect(onUpdateVisibility).toHaveBeenCalledTimes(1)
})

test('calls onToggleVisibility when checkbox clicked', () => {
  const onToggleVisibility = vi.fn()
  render(
    <SettingsTab
      visible={true}
      saving={false}
      onUpdateVisibility={vi.fn()}
      onToggleVisibility={onToggleVisibility}
      onDelete={vi.fn()}
    />
  )
  fireEvent.click(screen.getByRole('checkbox'))
  expect(onToggleVisibility).toHaveBeenCalledWith(false)
})

test('calls onDelete when Delete Problem clicked', () => {
  const onDelete = vi.fn()
  render(
    <SettingsTab
      visible={true}
      saving={false}
      onUpdateVisibility={vi.fn()}
      onToggleVisibility={vi.fn()}
      onDelete={onDelete}
    />
  )
  fireEvent.click(screen.getByText('Delete Problem'))
  expect(onDelete).toHaveBeenCalledTimes(1)
})
```

- [ ] **Step 2: Run test to verify it fails (RED)**

Run: `npx vitest run src/components/SetterWorkspace/SettingsTab.test.tsx --reporter=verbose`
Expected: FAIL

- [ ] **Step 3: Create SettingsTab component**

Write `web/src/components/SetterWorkspace/SettingsTab.tsx`:

```typescript
interface SettingsTabProps {
  visible: boolean
  saving: boolean
  onUpdateVisibility: () => void
  onToggleVisibility: (visible: boolean) => void
  onDelete: () => void
}

export default function SettingsTab({
  visible,
  saving,
  onUpdateVisibility,
  onToggleVisibility,
  onDelete,
}: SettingsTabProps) {
  return (
    <div className="space-y-6">
      <h2 className="text-lg font-bold text-gray-900 border-b pb-2 mb-4">Workspace Settings</h2>

      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <input
            type="checkbox"
            id="visible"
            checked={visible}
            onChange={e => onToggleVisibility(e.target.checked)}
            className="h-4 w-4 border-gray-300 rounded text-blue-600 focus:ring-blue-500"
          />
          <label htmlFor="visible" className="text-sm font-medium text-gray-700">
            Problem Visibility (Visible to Solvers)
          </label>
        </div>
        <p className="text-xs text-gray-400">
          If unchecked, the problem statement, statistics, and editorials will only be visible to
          authors and testers. Useful for preparing problems before contests.
        </p>

        <button
          onClick={onUpdateVisibility}
          disabled={saving}
          className="bg-blue-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-blue-700 transition-colors cursor-pointer"
        >
          Update Visibility
        </button>
      </div>

      <hr className="border-gray-200" />

      <div className="space-y-2">
        <h3 className="text-sm font-bold text-red-600 uppercase tracking-wider">Danger Zone</h3>
        <p className="text-xs text-gray-400">
          Deleting the problem will discard all statements, submissions history, and testcase files
          from the server sandbox. This operation cannot be undone.
        </p>
        <button
          onClick={onDelete}
          className="bg-red-600 text-white px-4 py-2 rounded text-sm font-semibold hover:bg-red-700 transition-colors cursor-pointer"
        >
          Delete Problem
        </button>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Run test to verify it passes (GREEN)**

Run: `npx vitest run src/components/SetterWorkspace/SettingsTab.test.tsx --reporter=verbose`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/components/SetterWorkspace/SettingsTab.tsx web/src/components/SetterWorkspace/SettingsTab.test.tsx
git commit -m "feat(workspace): extract SettingsTab sub-component"
```

---

## WAVE 2: Orchestrator Integration

### Task 2.1: Restructure the main SetterProblemWorkspace to use sub-components

**Files:**
- Modify: `web/src/pages/SetterProblemWorkspace.tsx`

### Scenario Contract
| Given | When | Then |
|-------|------|------|
| All sub-components exist | Import them in orchestrator | Compiles cleanly |
| Orchestrator uses sub-components | All 5 tabs render same content | Same functionality as before |
| Tab clicks | Swap active sub-component | Only active tab's content renders |

- [ ] **Step 1: Rewrite SetterProblemWorkspace.tsx to use sub-components**

Rewrite `web/src/pages/SetterProblemWorkspace.tsx` fully:

```typescript
import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api } from '../lib/api'
import type { ProblemFormState } from '../types/problem-workspace'
import type { Collaborator } from '../types/problem-workspace'
import StatementTab from '../components/SetterWorkspace/StatementTab'
import TestCasesTab from '../components/SetterWorkspace/TestCasesTab'
import CheckerTab from '../components/SetterWorkspace/CheckerTab'
import PermissionsTab from '../components/SetterWorkspace/PermissionsTab'
import SettingsTab from '../components/SetterWorkspace/SettingsTab'
import type { TestCase } from '../types/problem-workspace'

export default function SetterProblemWorkspace() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const [problem, setProblem] = useState<Record<string, unknown> | null>(null)
  const [activeTab, setActiveTab] = useState<'statement' | 'testcases' | 'checker' | 'permissions' | 'settings'>('statement')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  // Consolidated form state
  const [formState, setFormState] = useState<ProblemFormState>({
    title: '',
    description: '',
    inputFormat: '',
    outputFormat: '',
    hint: '',
    timeLimit: 1000,
    memoryLimit: 262144,
    difficulty: 'easy',
    tags: '',
    sampleCases: [],
    testcases: [],
    checkerType: 'exact',
    floatEpsilon: 1e-6,
    spj: false,
    spjLanguage: 'cpp-gpp-64',
    spjSourceCode: '',
    interactive: false,
    interactorLanguage: 'cpp-gpp-64',
    interactorSourceCode: '',
    visible: true,
  })

  // Test Cases supplementary state
  const [newTestCase, setNewTestCase] = useState<TestCase>({ input_name: '', output_name: '', score: 10 })
  const [batchScore, setBatchScore] = useState<number>(10)
  const [batchApplying, setBatchApplying] = useState(false)

  // Permissions supplementary state
  const [collaborators, setCollaborators] = useState<Collaborator[]>([])
  const [newUsername, setNewUsername] = useState('')
  const [newAccessLevel, setNewAccessLevel] = useState('co_author')

  const updateField = <K extends keyof ProblemFormState>(key: K, value: ProblemFormState[K]) => {
    setFormState(prev => ({ ...prev, [key]: value }))
  }

  const buildPayload = (overrides: Partial<ProblemFormState> = {}): Record<string, unknown> => {
    const merged = { ...formState, ...overrides }
    return {
      title: merged.title,
      description: merged.description,
      input_format: merged.inputFormat,
      output_format: merged.outputFormat,
      hint: merged.hint,
      time_limit: Number(merged.timeLimit),
      memory_limit: Number(merged.memoryLimit),
      difficulty: merged.difficulty,
      tags: merged.tags.split(',').map(t => t.trim()).filter(t => t !== ''),
      sample_cases: merged.sampleCases,
      testcase_score: merged.testcases,
      spj: merged.checkerType === 'custom',
      spj_language: merged.spjLanguage,
      spj_source_code: merged.spjSourceCode,
      checker_type: merged.checkerType,
      float_epsilon: merged.floatEpsilon,
      interactive: merged.interactive,
      interactor_language: merged.interactorLanguage,
      interactor_source_code: merged.interactorSourceCode,
      visible: merged.visible,
    }
  }

  const loadProblem = async () => {
    if (!slug) return
    try {
      const data = await api.problems.get(slug)
      setProblem(data)
      setFormState({
        title: data.title || '',
        description: data.description || '',
        inputFormat: data.input_format || '',
        outputFormat: data.output_format || '',
        hint: data.hint || '',
        timeLimit: data.time_limit || 1000,
        memoryLimit: data.memory_limit || 262144,
        difficulty: data.difficulty || 'easy',
        tags: data.tags?.join(', ') || '',
        sampleCases: data.sample_cases || [],
        testcases: data.testcase_score || [],
        checkerType: data.checker_type || 'exact',
        floatEpsilon: data.float_epsilon ?? 1e-6,
        spj: data.spj || false,
        spjLanguage: data.spj_language || 'cpp-gpp-64',
        spjSourceCode: data.spj_source_code || '',
        interactive: data.interactive || false,
        interactorLanguage: data.interactor_language || 'cpp-gpp-64',
        interactorSourceCode: data.interactor_source_code || '',
        visible: data.visible !== false,
      })

      const permData = await api.problems.getPermissions(data.slug)
      setCollaborators(permData.data || [])
    } catch (err: any) {
      setError(err.message || 'Failed to load problem workspace')
    }
  }

  useEffect(() => {
    loadProblem()
  }, [slug])

  const handleSave = async (overrides?: Partial<ProblemFormState>) => {
    setSaving(true)
    setError(null)
    setSuccess(null)
    try {
      const payload = buildPayload(overrides)
      await api.problems.update(problem?.slug as string, payload)
      setSuccess('Problem saved successfully!')
      loadProblem()
    } catch (err: any) {
      setError(err.message || 'Failed to save problem')
    } finally {
      setSaving(false)
    }
  }

  const handleSaveStatement = (e: React.FormEvent) => {
    e.preventDefault()
    handleSave()
  }

  const handleSaveChecker = () => {
    handleSave({ spj: formState.checkerType === 'custom' })
  }

  const handleAddTestCaseScore = async () => {
    if (!newTestCase.input_name || !newTestCase.output_name) {
      setError('Input and Output file names are required')
      return
    }
    setError(null)
    setSuccess(null)
    const updated = [...formState.testcases, newTestCase]
    setFormState(prev => ({ ...prev, testcases: updated }))
    setNewTestCase({ input_name: '', output_name: '', score: 10 })
    try {
      await api.problems.update(problem?.slug as string, buildPayload({ testcases: updated }))
      setSuccess('Testcase scores updated successfully!')
    } catch (err: any) {
      setError(err.message || 'Failed to save testcase scores')
    }
  }

  const handleRemoveTestCaseScore = async (index: number) => {
    setError(null)
    setSuccess(null)
    const updated = formState.testcases.filter((_, i) => i !== index)
    setFormState(prev => ({ ...prev, testcases: updated }))
    try {
      await api.problems.update(problem?.slug as string, buildPayload({ testcases: updated }))
      setSuccess('Testcase score removed successfully!')
    } catch (err: any) {
      setError(err.message || 'Failed to save testcase scores')
    }
  }

  const handleBatchSetScores = async () => {
    if (formState.testcases.length === 0) {
      setError('No testcases registered to allocate scores')
      return
    }
    setError(null)
    setSuccess(null)
    setBatchApplying(true)
    const updated = formState.testcases.map(tc => ({ ...tc, score: batchScore }))
    setFormState(prev => ({ ...prev, testcases: updated }))
    try {
      await api.problems.update(problem?.slug as string, buildPayload({ testcases: updated }))
      setSuccess(`All ${formState.testcases.length} testcase scores set to ${batchScore} points successfully!`)
    } catch (err: any) {
      setError(err.message || 'Failed to update testcase scores')
    } finally {
      setBatchApplying(false)
    }
  }

  const handleUploadTestcases = async (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!e.target.files || e.target.files.length === 0 || !problem) return
    setError(null)
    setSuccess(null)
    const file = e.target.files[0]
    try {
      await api.problems.uploadTestcases(problem.slug as string, file)
      setSuccess('Testcase package uploaded successfully!')
      loadProblem()
    } catch (err: any) {
      setError(err.message || 'Failed to upload testcase package')
    }
  }

  const handleAddCollaborator = async () => {
    if (!newUsername.trim() || !problem) return
    setError(null)
    setSuccess(null)
    try {
      await api.problems.addPermission(problem.slug as string, newUsername, newAccessLevel)
      setSuccess(`Collaborator ${newUsername} added successfully!`)
      setNewUsername('')
      loadProblem()
    } catch (err: any) {
      setError(err.message || 'Failed to add collaborator')
    }
  }

  const handleRemoveCollaborator = async (userId: string) => {
    if (!problem) return
    setError(null)
    setSuccess(null)
    try {
      await api.problems.removePermission(problem.slug as string, userId)
      setSuccess('Collaborator removed successfully!')
      loadProblem()
    } catch (err: any) {
      setError(err.message || 'Failed to remove collaborator')
    }
  }

  const handleDeleteProblem = async () => {
    if (!problem) return
    if (!window.confirm('Are you absolutely sure you want to delete this problem? This action CANNOT be undone.')) return
    setError(null)
    try {
      await api.problems.delete(problem.slug as string)
      navigate('/setter')
    } catch (err: any) {
      setError(err.message || 'Failed to delete problem')
    }
  }

  const handleUpdateVisibility = () => {
    handleSave({ visible: formState.visible })
  }

  if (!problem) {
    return <div className="text-center py-20 text-gray-400">Loading problem workspace...</div>
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-gray-200 pb-4">
        <div>
          <span className="text-xs text-gray-400 uppercase tracking-wider">Polygon Workspace</span>
          <h1 className="text-2xl font-bold text-gray-900">{problem.title as string}</h1>
        </div>
        <div className="flex gap-3">
          <Link
            to={`/problems/${problem.slug as string}`}
            className="border border-gray-300 text-gray-700 px-4 py-2 rounded text-sm hover:bg-gray-50 transition-colors"
          >
            View Public
          </Link>
          <Link
            to="/setter"
            className="bg-gray-100 border border-gray-200 text-gray-700 px-4 py-2 rounded text-sm hover:bg-gray-200 transition-colors"
          >
            Back to Setter Workspace
          </Link>
        </div>
      </div>

      {/* Status messages */}
      {error && (
        <div className="bg-red-50 border-l-4 border-red-500 p-4 text-sm text-red-700 rounded-r">
          {error}
        </div>
      )}
      {success && (
        <div className="bg-green-50 border-l-4 border-green-500 p-4 text-sm text-green-700 rounded-r">
          {success}
        </div>
      )}

      {/* Main content */}
      <div className="flex gap-6 items-start">
        {/* Tabs Sidebar */}
        <div className="w-56 shrink-0 flex flex-col border border-gray-200 rounded-lg bg-white overflow-hidden text-sm">
          <button
            onClick={() => setActiveTab('statement')}
            className={`px-4 py-3 text-left border-b border-gray-100 font-medium ${activeTab === 'statement' ? 'bg-blue-50 text-blue-600 border-l-4 border-l-blue-600' : 'text-gray-600 hover:bg-gray-50 hover:text-black'}`}
          >
            Statement & Details
          </button>
          <button
            onClick={() => setActiveTab('testcases')}
            className={`px-4 py-3 text-left border-b border-gray-100 font-medium ${activeTab === 'testcases' ? 'bg-blue-50 text-blue-600 border-l-4 border-l-blue-600' : 'text-gray-600 hover:bg-gray-50 hover:text-black'}`}
          >
            Test Cases / Data
          </button>
          <button
            onClick={() => setActiveTab('checker')}
            className={`px-4 py-3 text-left border-b border-gray-100 font-medium ${activeTab === 'checker' ? 'bg-blue-50 text-blue-600 border-l-4 border-l-blue-600' : 'text-gray-600 hover:bg-gray-50 hover:text-black'}`}
          >
            Checker / Special Judge
          </button>
          <button
            onClick={() => setActiveTab('permissions')}
            className={`px-4 py-3 text-left border-b border-gray-100 font-medium ${activeTab === 'permissions' ? 'bg-blue-50 text-blue-600 border-l-4 border-l-blue-600' : 'text-gray-600 hover:bg-gray-50 hover:text-black'}`}
          >
            Collaborators
          </button>
          <button
            onClick={() => setActiveTab('settings')}
            className={`px-4 py-3 text-left font-medium ${activeTab === 'settings' ? 'bg-blue-50 text-blue-600 border-l-4 border-l-blue-600' : 'text-gray-600 hover:bg-gray-50 hover:text-black'}`}
          >
            Workspace Settings
          </button>
        </div>

        {/* Tab Content Panels */}
        <div className="flex-1 bg-white border border-gray-200 rounded-lg p-6 min-h-[500px]">
          {activeTab === 'statement' && (
            <StatementTab
              formState={formState}
              saving={saving}
              onUpdate={updateField}
              onSave={handleSaveStatement}
            />
          )}

          {activeTab === 'testcases' && (
            <TestCasesTab
              testcases={formState.testcases}
              newTestCase={newTestCase}
              batchScore={batchScore}
              batchApplying={batchApplying}
              onAdd={handleAddTestCaseScore}
              onRemove={handleRemoveTestCaseScore}
              onBatchSet={handleBatchSetScores}
              onUpload={handleUploadTestcases}
              onNewTestCaseChange={setNewTestCase}
              onBatchScoreChange={setBatchScore}
            />
          )}

          {activeTab === 'checker' && (
            <CheckerTab
              formState={formState}
              saving={saving}
              onUpdate={updateField}
              onSave={handleSaveChecker}
            />
          )}

          {activeTab === 'permissions' && (
            <PermissionsTab
              collaborators={collaborators}
              newUsername={newUsername}
              newAccessLevel={newAccessLevel}
              onAdd={handleAddCollaborator}
              onRemove={handleRemoveCollaborator}
              onNewUsernameChange={setNewUsername}
              onNewAccessLevelChange={setNewAccessLevel}
            />
          )}

          {activeTab === 'settings' && (
            <SettingsTab
              visible={formState.visible}
              saving={saving}
              onUpdateVisibility={handleUpdateVisibility}
              onToggleVisibility={(v) => updateField('visible', v)}
              onDelete={handleDeleteProblem}
            />
          )}
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Run TypeScript compilation check**

Run: `npx tsc -b --noEmit`
Expected: Zero compilation errors.

- [ ] **Step 3: Run full test suite**

Run: `npx vitest run --reporter=verbose`
Expected: All tests pass:
- All 5 tab sub-component tests
- The main `SetterProblemWorkspace.test.tsx` tests
- Any pre-existing tests

- [ ] **Step 4: Remove .gitkeep placeholder**

Run: `rm web/src/components/SetterWorkspace/.gitkeep`

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/SetterProblemWorkspace.tsx web/src/components/SetterWorkspace/.gitkeep
git rm web/src/components/SetterWorkspace/.gitkeep
git commit -m "refactor(workspace): integrate sub-components into orchestrator, remove 800+ lines"
```

---

## WAVE 3: Cleanup & Verification

### Task 3.1: Final compilation and full test sweep

- [ ] **Step 1: Run TypeScript build check**

Run: `npx tsc -b --noEmit`
Expected: Zero errors.

- [ ] **Step 2: Run the complete test suite**

Run: `npx vitest run --reporter=verbose`
Expected: ALL tests PASS.

- [ ] **Step 3: Run lint**

Run: `npx eslint src/ --max-warnings 0`
Expected: No lint errors (verify eslint config allows the patterns used).

If lint fails, fix any issues (likely unused imports, prefer-const, etc.).

- [ ] **Step 4: Final verification of file sizes**

Run: `wc -l web/src/pages/SetterProblemWorkspace.tsx web/src/components/SetterWorkspace/*.tsx`
Expected:
- `SetterProblemWorkspace.tsx` — ~200 lines (down from 1101)
- Each sub-component — 100-300 lines

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: final cleanup after workspace restructure"
```

---

## Plan Self-Review

### Spec Coverage
| Spec Requirement | Task(s) |
|-----------------|---------|
| Types/ProblemFormState interface | 0.1 |
| Eliminate 5x payload duplication | 0.3 (buildPayload) |
| Sub-components for each tab | 1.1-1.5 |
| StatementTab | 1.1 |
| TestCasesTab | 1.2 |
| CheckerTab | 1.3 |
| PermissionsTab | 1.4 |
| SettingsTab | 1.5 |
| Orchestrator integration | 2.1 |
| TDD workflow (RED-GREEN-SURFACE) | Every task |
| Zero compilation errors | 0.1-3.1 |
| Preserve existing styling | All components (identical Tailwind classes) |

### Placeholder Check
- [x] No "TBD", "TODO", or incomplete code blocks
- [x] Every step has actual code content, not descriptions
- [x] No "similar to" references — each component has its own complete code
- [x] All file paths are exact
- [x] All commands are exact with expected output
- [x] All function/type names are consistent between tasks

### Type Consistency
- `ProblemFormState` — defined in 0.1, used by all tasks
- `updateField<K extends keyof ProblemFormState>` — defined in 2.1, consistent with `onUpdate` prop types
- `buildPayload()` — defined in 0.3, returns `Record<string, unknown>`
- `TestCase`, `Collaborator` — defined in 0.1, used in testcase and permissions tabs
