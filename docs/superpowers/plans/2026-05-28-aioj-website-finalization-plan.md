# AIOJ Website Finalization — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** Polish the AIOJ frontend by creating missing Problem Creator/Editor pages, a functional Submissions page, Profile settings, Setter application UI, and fixing all non-standard fetch calls.

**Architecture:** React + TypeScript + Tailwind frontend. All backend APIs already exist. Plan focuses on frontend-only work (no backend changes needed) and integrates existing `api.ts` client into all pages.

**Tech Stack:** React 19, TypeScript, Tailwind CSS, react-router-dom, CodeMirror (existing dependency).

---

### Task 1: Create Problem Creator Page (`/setter/create`)

**Files:**
- Create: `web/src/pages/ProblemCreate.tsx`
- Modify: `web/src/App.tsx` (add route + import)

- [ ] **Step 1: Write ProblemCreate.tsx**

```tsx
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../lib/api'

export default function ProblemCreate() {
    const nav = useNavigate()
    const [form, setForm] = useState({
        slug: '', title: '', description: '', difficulty: 'easy',
        time_limit: 1000, memory_limit: 262144, tags: '', input_format: '', output_format: '',
    })
    const [error, setError] = useState('')
    const [submitting, setSubmitting] = useState(false)

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        setError('')
        setSubmitting(true)
        try {
            await api.problems.create({
                slug: form.slug,
                title: form.title,
                description: form.description,
                difficulty: form.difficulty,
                time_limit: form.time_limit,
                memory_limit: form.memory_limit,
                tags: form.tags.split(',').map(t => t.trim()).filter(Boolean),
                input_format: form.input_format,
                output_format: form.output_format,
            })
            nav('/setter')
        } catch (err: any) {
            setError(err.message || 'Failed to create problem')
        } finally {
            setSubmitting(false)
        }
    }

    return <div className="max-w-2xl mx-auto">
        <h1 className="text-2xl font-bold mb-6">Create Problem</h1>
        {error && <div className="bg-red-50 border border-red-200 text-red-700 px-4 py-2 rounded mb-4 text-sm">{error}</div>}
        <form onSubmit={handleSubmit} className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Slug</label>
                    <input required value={form.slug} onChange={e => setForm(p => ({...p, slug: e.target.value}))}
                        className="w-full border rounded px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500" />
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Title</label>
                    <input required value={form.title} onChange={e => setForm(p => ({...p, title: e.target.value}))}
                        className="w-full border rounded px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500" />
                </div>
            </div>
            <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
                <textarea required rows={6} value={form.description}
                    onChange={e => setForm(p => ({...p, description: e.target.value}))}
                    className="w-full border rounded px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500" />
            </div>
            <div className="grid grid-cols-3 gap-4">
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Difficulty</label>
                    <select value={form.difficulty} onChange={e => setForm(p => ({...p, difficulty: e.target.value}))}
                        className="w-full border rounded px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500">
                        <option value="easy">Easy</option><option value="medium">Medium</option><option value="hard">Hard</option>
                    </select>
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Time Limit (ms)</label>
                    <input type="number" value={form.time_limit} onChange={e => setForm(p => ({...p, time_limit: +e.target.value}))}
                        className="w-full border rounded px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500" />
                </div>
                <div>
                    <label className="block text-sm font-medium text-gray-700 mb-1">Memory Limit (KB)</label>
                    <input type="number" value={form.memory_limit} onChange={e => setForm(p => ({...p, memory_limit: +e.target.value}))}
                        className="w-full border rounded px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500" />
                </div>
            </div>
            <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Tags (comma-separated)</label>
                <input value={form.tags} onChange={e => setForm(p => ({...p, tags: e.target.value}))}
                    placeholder="e.g. dp, graph, math"
                    className="w-full border rounded px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500" />
            </div>
            <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Input Format</label>
                <textarea rows={2} value={form.input_format}
                    onChange={e => setForm(p => ({...p, input_format: e.target.value}))}
                    className="w-full border rounded px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500" />
            </div>
            <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Output Format</label>
                <textarea rows={2} value={form.output_format}
                    onChange={e => setForm(p => ({...p, output_format: e.target.value}))}
                    className="w-full border rounded px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500" />
            </div>
            <button type="submit" disabled={submitting}
                className="bg-blue-600 text-white px-6 py-2 rounded hover:bg-blue-700 disabled:opacity-50 transition-colors">
                {submitting ? 'Creating...' : 'Create Problem'}
            </button>
        </form>
    </div>
}
```

- [ ] **Step 2: Update App.tsx**
Add import: `import ProblemCreate from './pages/ProblemCreate'`
Add route: `<Route path="/setter/create" element={<ProblemCreate />} />`

- [ ] **Step 3: Verify & Commit**
```bash
cd /Users/tahsinarafat/App_Dev/AIOJ/web && npm run build
cd .. && git add web/src && git commit -m "feat: add problem creator page at /setter/create"
```

---

### Task 2: Create Submissions Page (`/submissions`)

**Files:**
- Create: `web/src/pages/Submissions.tsx`
- Modify: `web/src/App.tsx` (replace stub route)

- [ ] **Step 1: Write Submissions.tsx**

```tsx
import { useEffect, useState } from 'react'
import { api } from '../lib/api'

const STATUS_COLORS: Record<string, string> = {
    ac: 'text-green-600', wa: 'text-red-600', tle: 'text-yellow-600',
    mle: 'text-orange-600', re: 'text-red-700', ce: 'text-purple-600',
    pending: 'text-blue-500', judging: 'text-blue-600', se: 'text-gray-600',
}

export default function Submissions() {
    const [subs, setSubs] = useState<any[]>([])
    const [total, setTotal] = useState(0)
    const [offset, setOffset] = useState(0)
    const limit = 20

    useEffect(() => {
        api.submissions.list(offset, limit).then(d => {
            setSubs(d.data || [])
            setTotal(d.total || 0)
        }).catch(console.error)
    }, [offset])

    return <div>
        <h1 className="text-2xl font-bold mb-4">My Submissions</h1>
        <div className="border border-gray-200 rounded-lg overflow-hidden">
            <table className="w-full text-sm">
                <thead className="bg-gray-50 text-gray-500 text-xs uppercase">
                    <tr>
                        <th className="px-4 py-3 text-left">Problem</th>
                        <th className="px-4 py-3 text-left">Language</th>
                        <th className="px-4 py-3 text-left">Verdict</th>
                        <th className="px-4 py-3 text-left">Time</th>
                        <th className="px-4 py-3 text-left">Memory</th>
                    </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                    {subs.map(s => (
                        <tr key={s.id} className="hover:bg-gray-50">
                            <td className="px-4 py-3 font-medium">{s.problem_id?.substring(0,8) || s.id.substring(0,8)}</td>
                            <td className="px-4 py-3 text-gray-500">{s.language}</td>
                            <td className="px-4 py-3 font-semibold"><span className={STATUS_COLORS[s.status] || ''}>{s.status}</span></td>
                            <td className="px-4 py-3 text-gray-500">{s.time_used > 0 ? `${s.time_used}ms` : '—'}</td>
                            <td className="px-4 py-3 text-gray-500">{s.memory_used > 0 ? `${Math.round(s.memory_used / 1024)}MB` : '—'}</td>
                        </tr>
                    ))}
                    {subs.length === 0 && <tr><td colSpan={5} className="px-4 py-8 text-center text-gray-400">No submissions yet.</td></tr>}
                </tbody>
            </table>
        </div>
        <div className="flex justify-between mt-4 text-sm text-gray-500">
            <span>{total} submissions</span>
            <div className="flex gap-2">
                <button onClick={() => setOffset(Math.max(0, offset - limit))} disabled={offset === 0}
                    className="px-3 py-1 border rounded disabled:opacity-40 hover:bg-gray-50">Prev</button>
                <button onClick={() => setOffset(offset + limit)} disabled={offset + limit >= total}
                    className="px-3 py-1 border rounded disabled:opacity-40 hover:bg-gray-50">Next</button>
            </div>
        </div>
    </div>
}
```

- [ ] **Step 2: Update App.tsx**
Add import: `import Submissions from './pages/Submissions'`
Replace the stub route: `<Route path="/submissions" element={<Submissions />} />`

- [ ] **Step 3: Verify & Commit**
```bash
cd web && npm run build && cd .. && git add web/src && git commit -m "feat: add submissions page with pagination"
```

---

### Task 3: Replace Raw Fetch with api Client in Contest Pages

**Files:**
- Modify: `web/src/pages/ContestList.tsx`
- Modify: `web/src/pages/ContestDetail.tsx`
- Modify: `web/src/pages/ContestScoreboard.tsx`
- Modify: `web/src/lib/api.ts` (add contest API methods if missing)

- [ ] **Step 1: Add contest API methods to api.ts**
```ts
// Add to api object in api.ts:
contests: {
    list: (offset = 0, limit = 20) =>
        request<{ data: any[]; total: number }>(`/contests?offset=${offset}&limit=${limit}`),
    get: (id: string) => request<any>(`/contests/${id}`),
    scoreboard: (id: string) => request<any>(`/contests/${id}/scoreboard`),
    create: (d: any) => request<any>('/contests', { method: 'POST', body: JSON.stringify(d) }),
    register: (id: string) => request(`/contests/${id}/register`, { method: 'POST' }),
    checkRegistration: (id: string) => request<{ registered: boolean }>(`/contests/${id}/register`),
},
```

- [ ] **Step 2: Replace raw fetch in ContestList.tsx**
Replace `fetch('/api/contests')` with `api.contests.list()`. Update the data access pattern to match the api response.

- [ ] **Step 3: Replace raw fetch in ContestDetail.tsx**
Replace `fetch('/api/contests/${id}')` with `api.contests.get(id)`. Replace manual registration fetch with `api.contests.checkRegistration(id)` and `api.contests.register(id)`.

- [ ] **Step 4: Replace raw fetch in ContestScoreboard.tsx**
Replace `fetch('/api/contests/${id}/scoreboard')` with `api.contests.scoreboard(id)`.

- [ ] **Step 5: Verify & Commit**
```bash
cd web && npm run build && cd .. && git add web/src && git commit -m "fix: replace raw fetch with api client in contest pages"
```

---

### Task 4: Profile Settings Page (`/profile`)

**Files:**
- Modify: `web/src/App.tsx` (add import + route + navbar link)

- [ ] **Step 1: Write Profile.tsx**

```tsx
import { useState } from 'react'
import { api, getAccessToken } from '../lib/api'

function decodeUser() {
    const token = getAccessToken()
    if (!token) return null
    try {
        return JSON.parse(atob(token.split('.')[1]))
    } catch { return null }
}

export default function Profile() {
    const user = decodeUser()

    return <div className="max-w-md mx-auto">
        <h1 className="text-2xl font-bold mb-6">Profile Settings</h1>

        <div className="space-y-4 bg-white border border-gray-200 rounded-lg p-6">
            <div>
                <label className="block text-sm text-gray-500 mb-1">Username</label>
                <p className="font-medium">{user?.uname || '—'}</p>
            </div>
            <div>
                <label className="block text-sm text-gray-500 mb-1">Role</label>
                <p className="font-medium">{user?.role || 'user'}</p>
            </div>
            <div>
                <label className="block text-sm text-gray-500 mb-1">User ID</label>
                <p className="font-mono text-xs text-gray-400">{user?.uid || '—'}</p>
            </div>
        </div>

        <section className="mt-8">
            <h2 className="text-lg font-semibold mb-4">Setter Application</h2>
            <SetterApplication />
        </section>
    </div>
}

function SetterApplication() {
    const [status, setStatus] = useState<string | null>(null)
    const [reason, setReason] = useState('')
    const [submitted, setSubmitted] = useState(false)

    useState(() => {
        api.setter.status().then(d => setStatus(d?.status || null)).catch(() => {})
    })

    const handleApply = async () => {
        await api.setter.apply(reason)
        setSubmitted(true)
        setStatus('pending')
    }

    if (status === 'approved') return <p className="text-green-600">✅ You are a problem setter!</p>
    if (status === 'pending') return <p className="text-yellow-600">⏳ Your application is pending review.</p>

    return <div>
        {status === 'rejected' && <p className="text-red-600 mb-2">Your previous application was rejected. You can re-apply.</p>}
        {!submitted ? (
            <div>
                <textarea rows={3} value={reason} onChange={e => setReason(e.target.value)}
                    placeholder="Why do you want to become a setter?"
                    className="w-full border rounded px-3 py-2 text-sm mb-2 focus:ring-2 focus:ring-blue-500" />
                <button onClick={handleApply} disabled={!reason.trim()}
                    className="bg-blue-600 text-white px-4 py-2 rounded text-sm hover:bg-blue-700 disabled:opacity-50">
                    Apply as Problem Setter
                </button>
            </div>
        ) : (
            <p className="text-green-600">Application submitted!</p>
        )}
    </div>
}
```

- [ ] **Step 2: Update App.tsx**
Add import + route: `<Route path="/profile" element={<Profile />} />`
Add navbar link in logged-in section: `<Link to="/profile" className="text-sm text-gray-600 hover:text-black">Profile</Link>`

- [ ] **Step 3: Verify & Commit**
```bash
cd web && npm run build && cd .. && git add web/src && git commit -m "feat: add profile page and setter application UI"
```

---

### Task 5: Final Smoke Test

- [ ] **Step 1:** Build frontend: `cd /Users/tahsinarafat/App_Dev/AIOJ/web && npm run build`
- [ ] **Step 2:** Build backend: `cd /Users/tahsinarafat/App_Dev/AIOJ && go build ./...`
- [ ] **Step 3:** Restart Docker (to serve new frontend build): `cd /Users/tahsinarafat/App_Dev/AIOJ && docker compose build frontend && docker compose up -d --force-recreate frontend`
- [ ] **Step 4:** Manual QA: visit `/setter/create`, `/submissions`, `/profile`, click `+ Create Problem` from SetterWorkspace, and verify all pages load without 404.

---
