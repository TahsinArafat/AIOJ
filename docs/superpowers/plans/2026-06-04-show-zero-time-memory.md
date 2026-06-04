# Show Zero Time and Memory Instead of Dash Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show `0ms` / `0MB` instead of `—` (dash) for time and memory when the value is exactly 0, except when the submission is pending, judging, has compile error, or system error.

**Architecture:** Modify the frontend UI page files to render time and memory when the submission is in a completed/judged state (e.g. AC, WA, TLE, MLE, RE). If the submission is pending, judging, or has errors, continue showing the em dash `—`.

**Tech Stack:** React, TypeScript, Tailwind CSS

---

### Task 1: Update Submissions List View

**Files:**
- Modify: `web/src/pages/Submissions.tsx`

- [ ] **Step 1: Locate time/memory columns in submissions table**

Open `web/src/pages/Submissions.tsx` and locate lines 58-59.

- [ ] **Step 2: Update conditional rendering**

Modify lines 58-59 to display time and memory when the status is not pending, judging, ce, or se:

```tsx
<td className="px-4 py-3 text-gray-500 dark:text-gray-400">
    {s.status !== 'pending' && s.status !== 'judging' && s.status !== 'ce' && s.status !== 'se' ? `${s.time_used}ms` : '—'}
</td>
<td className="px-4 py-3 text-gray-500 dark:text-gray-400">
    {s.status !== 'pending' && s.status !== 'judging' && s.status !== 'ce' && s.status !== 'se' ? `${Math.round(s.memory_used / 1024)}MB` : '—'}
</td>
```

---

### Task 2: Update Submission Detail Page

**Files:**
- Modify: `web/src/pages/SubmissionDetail.tsx`

- [ ] **Step 1: Locate time/memory elements in overview tab and testcase list**

Open `web/src/pages/SubmissionDetail.tsx` and check:
- Lines 183 and 187 (overview tab)
- Lines 253-254 (subtask testcases)
- Lines 284-285 (fallback non-subtask testcases)

- [ ] **Step 2: Update overview tab rendering**

Modify lines 183 and 187:

```tsx
<p className="text-xl font-bold mt-1">
    {sub.status !== 'pending' && sub.status !== 'judging' && sub.status !== 'ce' && sub.status !== 'se' ? `${sub.time_used}ms` : '—'}
</p>
```
and
```tsx
<p className="text-xl font-bold mt-1">
    {sub.status !== 'pending' && sub.status !== 'judging' && sub.status !== 'ce' && sub.status !== 'se' ? `${Math.round(sub.memory_used / 1024)}MB` : '—'}
</p>
```

- [ ] **Step 3: Update subtask individual testcases rendering**

Modify lines 253-254:

```tsx
{c.time !== undefined && c.time !== null && <span>Time: {c.time}ms</span>}
{c.memory !== undefined && c.memory !== null && <span>Memory: {Math.round(c.memory / 1024)}MB</span>}
```

- [ ] **Step 4: Update fallback non-subtask individual testcases rendering**

Modify lines 284-285:

```tsx
<span>Time: {r.time !== undefined && r.time !== null ? `${r.time}ms` : '—'}</span>
<span>Memory: {r.memory !== undefined && r.memory !== null ? `${Math.round(r.memory / 1024)}MB` : '—'}</span>
```

---

### Task 3: Build Web Frontend to Verify

- [ ] **Step 1: Run Web Build**

Run: `npm run build` inside `web` directory.
Expected: Build succeeds with zero TypeScript or build compile errors.
