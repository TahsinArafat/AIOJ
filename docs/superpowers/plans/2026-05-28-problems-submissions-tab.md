# Problem Submissions Tab & Polygon Setter Refinements Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Overhaul the problem detail view layout by adding a persistent, interactive "My Submissions" tab directly alongside the problem description, statistics, and editorials tabs, while strengthening Polygon setter validations.

**Architecture:** Inject a reactive `submissions` tab view in React router paths for `/problems/:slug`, fetch sub-lists filtered by `problem_id` on selection/polling updates, support side-by-side upsolving list details, and integrate automated UI testing blocks with Vitest.

**Tech Stack:** React 19, TypeScript, Tailwind CSS, Vitest, Testing Library.

---

### Task 1: Extend Submissions API Filter and Frontend Type Definitions

**Files:**
- Modify: `web/src/lib/api.ts:111-113`
- Test: `web/src/test/setup.ts`

- [ ] **Step 1: Check existing api.ts submissions implementation**

Verify line 111-113:
```typescript
        list: (offset = 0, limit = 20) =>
            request<{ data: any[]; total: number }>(`/submissions?offset=${offset}&limit=${limit}`),
```

- [ ] **Step 2: Modify submissions.list signature to accept problem filter**

Rewrite the submission list helper inside `web/src/lib/api.ts` to accept optional `problemId` and contest filters:
```typescript
        list: (offset = 0, limit = 20, problemId?: string, contestId?: string) => {
            let url = `/submissions?offset=${offset}&limit=${limit}`;
            if (problemId) url += `&problem_id=${problemId}`;
            if (contestId) url += `&contest_id=${contestId}`;
            return request<{ data: any[]; total: number }>(url);
        },
```

- [ ] **Step 3: Compile check frontend workspace**

Run: `npm run build`
Expected: PASS with compiled assets.

---

### Task 2: Implement "My Submissions" Tab in Problem Detail Page

**Files:**
- Modify: `web/src/pages/ProblemDetail.tsx`

- [ ] **Step 1: Read current tab states inside ProblemDetail.tsx**

Check state at line 46:
```typescript
    const [tab, setTab] = useState<'statement' | 'stats' | 'editorials'>('statement')
```

- [ ] **Step 2: Add submissions state and extends tab type**

Modify the types and state to support `submissions` tab:
```typescript
    const [tab, setTab] = useState<'statement' | 'stats' | 'editorials' | 'submissions'>('statement')
    const [mySubs, setMySubs] = useState<any[]>([])
    const [loadingSubs, setLoadingSubs] = useState(false)
```

- [ ] **Step 3: Fetch submissions for active problem**

Add a `useEffect` hook to pull submissions list when selecting the submissions tab:
```typescript
    useEffect(() => {
        if (tab === 'submissions' && problem?.id) {
            setLoadingSubs(true)
            api.submissions.list(0, 50, problem.id, contestId || undefined)
                .then(d => setMySubs(d.data || []))
                .catch(console.error)
                .finally(() => setLoadingSubs(false))
        }
    }, [tab, problem?.id, contestId])
```

- [ ] **Step 4: Render Submissions tab selector and table inside UI**

Inject the tab button:
```typescript
                    <button onClick={() => setTab('submissions')} className={`px-4 py-2 text-sm font-medium border-b-2 -mb-px ${tab === 'submissions' ? 'border-blue-600 text-blue-600' : 'border-transparent text-gray-500 hover:text-gray-700'}`}>
                        My Submissions
                    </button>
```

And define the render block in tab selectors:
```typescript
                ) : tab === 'submissions' ? (
                    <div className="space-y-4">
                        {loadingSubs ? (
                            <div className="text-center py-8 text-gray-400">Loading submissions...</div>
                        ) : mySubs.length === 0 ? (
                            <p className="text-gray-400 text-sm text-center py-8">No submissions yet for this problem.</p>
                        ) : (
                            <div className="border border-gray-200 rounded-lg overflow-hidden">
                                <table className="w-full text-sm">
                                    <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                                        <tr>
                                            <th className="px-4 py-2 text-left">ID</th>
                                            <th className="px-4 py-2 text-left">Language</th>
                                            <th className="px-4 py-2 text-left">Verdict</th>
                                            <th className="px-4 py-2 text-left">Time</th>
                                            <th className="px-4 py-2 text-left">Memory</th>
                                        </tr>
                                    </thead>
                                    <tbody className="divide-y divide-gray-100">
                                        {mySubs.map((s: any) => (
                                            <tr key={s.id} className="hover:bg-gray-50">
                                                <td className="px-4 py-2 font-mono text-xs text-blue-600 hover:underline">
                                                    <Link to={`/submissions/${s.id}`}>
                                                        {s.id?.substring(0, 8)}...
                                                    </Link>
                                                </td>
                                                <td className="px-4 py-2 text-gray-500">{s.language}</td>
                                                <td className="px-4 py-2 font-semibold">
                                                    <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                                                        s.status === 'ac' ? 'text-green-600 bg-green-50' : 'text-red-600 bg-red-50'
                                                    }`}>
                                                        {s.status === 'ac' ? 'Accepted' : s.status}
                                                    </span>
                                                </td>
                                                <td className="px-4 py-2 text-gray-500">{s.time_used}ms</td>
                                                <td className="px-4 py-2 text-gray-500">{Math.round(s.memory_used / 1024)}MB</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        )}
                    </div>
```

---

### Task 3: Compose Vitest Unit Verification tests for ProblemDetail

**Files:**
- Create: `web/src/pages/ProblemDetail.test.tsx`

- [ ] **Step 1: Write test case simulating submissions tab selection**

Create `web/src/pages/ProblemDetail.test.tsx`:
```typescript
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { expect, test, vi } from 'vitest'
import ProblemDetail from './ProblemDetail'
import { api } from '../lib/api'

vi.mock('../lib/api', () => ({
    api: {
        problems: {
            get: vi.fn().mockResolvedValue({
                id: 'prob-1',
                slug: 'two-sum',
                title: 'Two Sum',
                description: 'Solve the sum problem.',
                time_limit: 1000,
                memory_limit: 262144,
                difficulty: 'easy',
                sample_cases: []
            })
        },
        editorials: {
            getByProblem: vi.fn().mockResolvedValue({ data: [] })
        },
        submissions: {
            list: vi.fn().mockResolvedValue({
                data: [
                    { id: 'sub-1', language: 'cpp-gpp-64', status: 'ac', time_used: 12, memory_used: 2048 }
                ],
                total: 1
            })
        }
    },
    getAccessToken: vi.fn().mockReturnValue('mock-token')
}))

test('renders submissions tab and handles list load clicks', async () => {
    render(
        <MemoryRouter>
            <ProblemDetail />
        </MemoryRouter>
    )

    // Wait for description render
    await waitFor(() => {
        expect(screen.getByText('Two Sum')).toBeInTheDocument()
    })

    // Click Submissions Tab
    const subTab = screen.getByText('My Submissions')
    expect(subTab).toBeInTheDocument()
    fireEvent.click(subTab)

    // Wait for submission details to load
    await waitFor(() => {
        expect(api.submissions.list).toHaveBeenCalled()
    })
})
```

- [ ] **Step 2: Run tests to verify the behavior passes**

Run: `npm run test`
Expected: PASS
