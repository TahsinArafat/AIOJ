# Sub-Plan 08: Upsolving

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to solve contest problems after the contest ends with full test feedback.

**Architecture:** Add upsolving mode to submissions, show full test results, track upsolving statistics.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Modify
- `internal/api/handler/submission.go` - Add upsolving mode
- `internal/store/postgres/submissions.go` - Add upsolving queries

### Frontend Files to Modify
- `web/src/pages/ProblemDetail.tsx` - Show upsolving mode
- `web/src/pages/ContestDetail.tsx` - Add upsolving link

---

## Tasks

### Task 1: Backend Upsolving Support

**Files:**
- Modify: `internal/api/handler/submission.go`

- [ ] **Step 1: Add upsolving submission handler**

```go
func (h *SubmissionHandler) CreateUpsolving(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	var req model.SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	// Verify contest has ended
	if req.ContestID != "" {
		contest, err := h.contestStore.GetByID(r.Context(), req.ContestID)
		if err != nil || contest == nil {
			http.Error(w, "contest not found", http.StatusNotFound)
			return
		}
		
		if time.Now().Before(contest.EndTime) {
			http.Error(w, "contest hasn't ended yet", http.StatusBadRequest)
			return
		}
	}
	
	// Create submission with full feedback
	submission := &model.Submission{
		ID:         uuid.New().String(),
		ProblemID:  req.ProblemID,
		UserID:     claims.UserID,
		ContestID:  req.ContestID,
		Language:   req.Language,
		SourceCode: req.SourceCode,
		CodeSize:   len(req.SourceCode),
		Status:     model.StatusPending,
	}
	
	if err := h.submissionStore.Create(r.Context(), submission); err != nil {
		http.Error(w, "submit failed", http.StatusInternalServerError)
		return
	}
	
	// Queue for judging with full feedback
	h.queue.Push(submission.ID)
	
	respondJSON(w, http.StatusCreated, submission)
}
```

- [ ] **Step 2: Add route**

```go
r.Post("/api/submissions/upsolving", submissionH.CreateUpsolving)
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/submission.go internal/api/router.go
git commit -m "feat(upsolving): add upsolving submission endpoint"
```

---

### Task 2: Frontend Upsolving Mode

**Files:**
- Modify: `web/src/pages/ContestDetail.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add upsolving API call**

```typescript
submissions: {
    // ... existing methods ...
    createUpsolving: (d: { problem_id: string; language: string; source_code: string; contest_id?: string }) =>
        request<any>('/submissions/upsolving', { method: 'POST', body: JSON.stringify(d) }),
},
```

- [ ] **Step 2: Add upsolving section to ContestDetail**

```tsx
// Add upsolving section for ended contests
{isEnded && (
  <div className="mt-6">
    <h2 className="text-lg font-semibold mb-3">Upsolving</h2>
    <p className="text-gray-600 mb-4">
      Practice contest problems with full test feedback.
    </p>
    <div className="space-y-2">
      {problems?.map((p: any) => (
        <Link
          key={p.problem_id}
          to={`/problems/${p.problem_id}?contest=${id}&upsolving=true`}
          className="block px-4 py-3 border rounded-lg hover:bg-gray-50"
        >
          <span className="font-medium">{p.index}. </span>
          Problem {p.problem_id}
        </Link>
      ))}
    </div>
  </div>
)}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ContestDetail.tsx web/src/lib/api.ts
git commit -m "feat(upsolving): add upsolving UI to contest detail"
```

---

## Verification Checklist

- [ ] Upsolving only works after contest ends
- [ ] Full test feedback shown
- [ ] Submissions marked as upsolving
- [ ] Upsolving link appears on ended contests

---

## Notes

1. **Full feedback**: All test cases shown (not just failed)
2. **No rating impact**: Upsolving doesn't affect rating
3. **Statistics**: Upsolving tracked separately
