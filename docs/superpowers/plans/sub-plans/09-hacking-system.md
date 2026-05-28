# Sub-Plan 09: Hacking System

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to hack (challenge) other participants' solutions by providing counter-test cases during/after contests.

**Architecture:** Add `hacks` table, hack service, hack scoring, frontend hack UI.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Create
- `internal/model/hack.go` - Hack models
- `internal/store/postgres/hacks.go` - Hack store
- `internal/hack/service.go` - Hack service
- `internal/hack/service_test.go` - Service tests
- `internal/api/handler/hack.go` - Hack handler

### Backend Files to Modify
- `internal/store/interfaces.go` - Add HackStore interface
- `internal/api/router.go` - Add hack routes

### Frontend Files to Create
- `web/src/pages/HackPanel.tsx` - Hack submission UI
- `web/src/components/HackResult.tsx` - Hack result display

### Frontend Files to Modify
- `web/src/App.tsx` - Add hack route
- `web/src/lib/api.ts` - Add hack API calls
- `web/src/pages/ContestDetail.tsx` - Add hack button

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000006_hacking_system.up.sql`
- Create: `internal/store/migrations/000006_hacking_system.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000006_hacking_system.up.sql

CREATE TABLE hacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id),
    problem_id UUID NOT NULL REFERENCES problems(id),
    hacker_id UUID NOT NULL REFERENCES users(id),
    defender_id UUID NOT NULL REFERENCES users(id),
    submission_id UUID NOT NULL REFERENCES submissions(id),
    test_input TEXT NOT NULL,
    expected_output TEXT,
    actual_output TEXT,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'success', 'failure', 'error')),
    success BOOLEAN,
    hacker_rating_change INTEGER NOT NULL DEFAULT 0,
    defender_rating_change INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    judged_at TIMESTAMPTZ
);

CREATE INDEX idx_hacks_contest ON hacks(contest_id);
CREATE INDEX idx_hacks_hacker ON hacks(hacker_id);
CREATE INDEX idx_hacks_defender ON hacks(defender_id);
CREATE INDEX idx_hacks_submission ON hacks(submission_id);

-- Add hack phase columns to contests
ALTER TABLE contests ADD COLUMN IF NOT EXISTS hack_phase_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS hack_phase_start TIMESTAMPTZ;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS hack_phase_end TIMESTAMPTZ;
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000006_hacking_system.down.sql

DROP TABLE IF EXISTS hacks;
ALTER TABLE contests DROP COLUMN IF EXISTS hack_phase_enabled;
ALTER TABLE contests DROP COLUMN IF EXISTS hack_phase_start;
ALTER TABLE contests DROP COLUMN IF EXISTS hack_phase_end;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: Migration applied successfully

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000006_hacking_system.*
git commit -m "feat(hack): add hacking system database migration"
```

---

### Task 2: Hack Models

**Files:**
- Create: `internal/model/hack.go`

- [ ] **Step 1: Create hack models**

```go
// internal/model/hack.go
package model

import "time"

type Hack struct {
	ID                 string     `json:"id"`
	ContestID          string     `json:"contest_id"`
	ProblemID          string     `json:"problem_id"`
	HackerID           string     `json:"hacker_id"`
	HackerUsername      string     `json:"hacker_username,omitempty"`
	DefenderID         string     `json:"defender_id"`
	DefenderUsername    string     `json:"defender_username,omitempty"`
	SubmissionID       string     `json:"submission_id"`
	TestInput          string     `json:"test_input"`
	ExpectedOutput     string     `json:"expected_output,omitempty"`
	ActualOutput       string     `json:"actual_output,omitempty"`
	Status             string     `json:"status"`
	Success            *bool      `json:"success,omitempty"`
	HackerRatingChange int        `json:"hacker_rating_change"`
	DefenderRatingChange int      `json:"defender_rating_change"`
	CreatedAt          time.Time  `json:"created_at"`
	JudgedAt           *time.Time `json:"judged_at,omitempty"`
}

type HackRequest struct {
	ContestID    string `json:"contest_id"`
	ProblemID    string `json:"problem_id"`
	SubmissionID string `json:"submission_id"`
	TestInput    string `json:"test_input"`
}

type HackResult struct {
	HackID         string `json:"hack_id"`
	Status         string `json:"status"`
	Success        bool   `json:"success"`
	ExpectedOutput string `json:"expected_output,omitempty"`
	ActualOutput   string `json:"actual_output,omitempty"`
	Message        string `json:"message,omitempty"`
}

type HackStats struct {
	TotalHacks      int `json:"total_hacks"`
	SuccessfulHacks int `json:"successful_hacks"`
	FailedHacks     int `json:"failed_hacks"`
	HackPoints      int `json:"hack_points"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/hack.go
git commit -m "feat(hack): add hack models"
```

---

### Task 3: Hack Store

**Files:**
- Create: `internal/store/postgres/hacks.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Add HackStore interface**

Add to `internal/store/interfaces.go`:

```go
type HackStore interface {
	Create(ctx context.Context, h *model.Hack) error
	GetByID(ctx context.Context, id string) (*model.Hack, error)
	UpdateStatus(ctx context.Context, id, status string, success bool) error
	GetByContest(ctx context.Context, contestID string) ([]model.Hack, error)
	GetByHacker(ctx context.Context, hackerID string) ([]model.Hack, error)
	GetByDefender(ctx context.Context, defenderID string) ([]model.Hack, error)
	GetHackableSubmissions(ctx context.Context, contestID, problemID string) ([]model.Submission, error)
}
```

- [ ] **Step 2: Implement hack store**

```go
// internal/store/postgres/hacks.go
package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type HackStore struct {
	db *sql.DB
}

func NewHackStore(db *sql.DB) *HackStore {
	return &HackStore{db: db}
}

func (s *HackStore) Create(ctx context.Context, h *model.Hack) error {
	return s.db.QueryRowContext(ctx,
		`INSERT INTO hacks (contest_id, problem_id, hacker_id, defender_id, submission_id, test_input)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		h.ContestID, h.ProblemID, h.HackerID, h.DefenderID, h.SubmissionID, h.TestInput,
	).Scan(&h.ID, &h.CreatedAt)
}

func (s *HackStore) GetByID(ctx context.Context, id string) (*model.Hack, error) {
	var h model.Hack
	err := s.db.QueryRowContext(ctx,
		`SELECT id, contest_id, problem_id, hacker_id, defender_id, submission_id,
		        test_input, expected_output, actual_output, status, success,
		        hacker_rating_change, defender_rating_change, created_at, judged_at
		 FROM hacks WHERE id = $1`,
		id).Scan(&h.ID, &h.ContestID, &h.ProblemID, &h.HackerID, &h.DefenderID,
		&h.SubmissionID, &h.TestInput, &h.ExpectedOutput, &h.ActualOutput,
		&h.Status, &h.Success, &h.HackerRatingChange, &h.DefenderRatingChange,
		&h.CreatedAt, &h.JudgedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (s *HackStore) UpdateStatus(ctx context.Context, id, status string, success bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE hacks SET status = $1, success = $2, judged_at = NOW() WHERE id = $3`,
		status, success, id)
	return err
}

func (s *HackStore) GetByContest(ctx context.Context, contestID string) ([]model.Hack, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT h.id, h.contest_id, h.problem_id, h.hacker_id, hu.username, h.defender_id, du.username,
		        h.submission_id, h.test_input, h.status, h.success, h.created_at
		 FROM hacks h
		 JOIN users hu ON h.hacker_id = hu.id
		 JOIN users du ON h.defender_id = du.id
		 WHERE h.contest_id = $1
		 ORDER BY h.created_at DESC`,
		contestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hacks []model.Hack
	for rows.Next() {
		var h model.Hack
		if err := rows.Scan(&h.ID, &h.ContestID, &h.ProblemID, &h.HackerID, &h.HackerUsername,
			&h.DefenderID, &h.DefenderUsername, &h.SubmissionID, &h.TestInput,
			&h.Status, &h.Success, &h.CreatedAt); err != nil {
			return nil, err
		}
		hacks = append(hacks, h)
	}
	if hacks == nil {
		hacks = []model.Hack{}
	}
	return hacks, nil
}

func (s *HackStore) GetHackableSubmissions(ctx context.Context, contestID, problemID string) ([]model.Submission, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT s.id, s.problem_id, s.user_id, s.language, s.status, s.created_at
		 FROM submissions s
		 WHERE s.contest_id = $1 AND s.problem_id = $2 AND s.status = 'ac'
		 ORDER BY s.created_at DESC`,
		contestID, problemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var submissions []model.Submission
	for rows.Next() {
		var s model.Submission
		if err := rows.Scan(&s.ID, &s.ProblemID, &s.UserID, &s.Language, &s.Status, &s.CreatedAt); err != nil {
			return nil, err
		}
		submissions = append(submissions, s)
	}
	return submissions, nil
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/hacks.go
git commit -m "feat(hack): add hack store"
```

---

### Task 4: Hack Service

**Files:**
- Create: `internal/hack/service.go`
- Create: `internal/hack/service_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/hack/service_test.go
package hack

import (
	"testing"
)

func TestValidateHackRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     HackValidation
		wantErr bool
	}{
		{
			name: "valid hack",
			req: HackValidation{
				TestInput:    "5\n",
				SubmissionID: "sub-123",
			},
			wantErr: false,
		},
		{
			name: "empty test input",
			req: HackValidation{
				TestInput:    "",
				SubmissionID: "sub-123",
			},
			wantErr: true,
		},
		{
			name: "empty submission ID",
			req: HackValidation{
				TestInput:    "5\n",
				SubmissionID: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHackRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHackRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: Implement hack service**

```go
// internal/hack/service.go
package hack

import (
	"context"
	"errors"
	"time"

	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store"
)

type HackValidation struct {
	TestInput    string
	SubmissionID string
}

type Service struct {
	hackStore    store.HackStore
	contestStore store.ContestStore
	judgeService JudgeService
}

type JudgeService interface {
	JudgeHack(ctx context.Context, submissionID, testInput string) (expected, actual string, success bool, err error)
}

func NewService(hs store.HackStore, cs store.ContestStore, js JudgeService) *Service {
	return &Service{
		hackStore:    hs,
		contestStore: cs,
		judgeService: js,
	}
}

func ValidateHackRequest(req HackValidation) error {
	if req.TestInput == "" {
		return errors.New("test input is required")
	}
	if req.SubmissionID == "" {
		return errors.New("submission ID is required")
	}
	return nil
}

// SubmitHack processes a hack request
func (s *Service) SubmitHack(ctx context.Context, hackerID string, req model.HackRequest) (*model.HackResult, error) {
	// Validate request
	if err := ValidateHackRequest(HackValidation{
		TestInput:    req.TestInput,
		SubmissionID: req.SubmissionID,
	}); err != nil {
		return nil, err
	}

	// Get submission to find defender
	submission, err := s.getSubmission(ctx, req.SubmissionID)
	if err != nil || submission == nil {
		return nil, errors.New("submission not found")
	}

	// Can't hack yourself
	if submission.UserID == hackerID {
		return nil, errors.New("cannot hack your own submission")
	}

	// Check if hack phase is active
	contest, err := s.contestStore.GetByID(ctx, req.ContestID)
	if err != nil || contest == nil {
		return nil, errors.New("contest not found")
	}

	if !s.isHackPhaseActive(contest) {
		return nil, errors.New("hack phase is not active")
	}

	// Create hack record
	hack := &model.Hack{
		ContestID:    req.ContestID,
		ProblemID:    req.ProblemID,
		HackerID:     hackerID,
		DefenderID:   submission.UserID,
		SubmissionID: req.SubmissionID,
		TestInput:    req.TestInput,
		Status:       "pending",
	}

	if err := s.hackStore.Create(ctx, hack); err != nil {
		return nil, err
	}

	// Judge the hack
	expected, actual, success, err := s.judgeService.JudgeHack(ctx, req.SubmissionID, req.TestInput)
	if err != nil {
		s.hackStore.UpdateStatus(ctx, hack.ID, "error", false)
		return &model.HackResult{
			HackID:  hack.ID,
			Status:  "error",
			Message: "Failed to judge hack",
		}, nil
	}

	// Update hack with results
	status := "failure"
	if success {
		status = "success"
	}
	s.hackStore.UpdateStatus(ctx, hack.ID, status, success)

	return &model.HackResult{
		HackID:         hack.ID,
		Status:         status,
		Success:        success,
		ExpectedOutput: expected,
		ActualOutput:   actual,
	}, nil
}

func (s *Service) isHackPhaseActive(contest *model.Contest) bool {
	if !contest.HackPhaseEnabled {
		return false
	}
	now := time.Now()
	if contest.HackPhaseStart != nil && now.Before(*contest.HackPhaseStart) {
		return false
	}
	if contest.HackPhaseEnd != nil && now.After(*contest.HackPhaseEnd) {
		return false
	}
	return true
}

func (s *Service) getSubmission(ctx context.Context, id string) (*model.Submission, error) {
	// Would need submission store
	return nil, nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/hack/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/hack/service.go internal/hack/service_test.go
git commit -m "feat(hack): add hack service"
```

---

### Task 5: Hack Handler

**Files:**
- Create: `internal/api/handler/hack.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create hack handler**

```go
// internal/api/handler/hack.go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/hack"
	"github.com/tahsinarafat/aioj/internal/model"
)

type HackHandler struct {
	service *hack.Service
}

func NewHackHandler(s *hack.Service) *HackHandler {
	return &HackHandler{service: s}
}

// SubmitHack submits a hack challenge
func (h *HackHandler) SubmitHack(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req model.HackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	result, err := h.service.SubmitHack(r.Context(), claims.UserID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetHack returns a specific hack
func (h *HackHandler) GetHack(w http.ResponseWriter, r *http.Request) {
	hackID := chi.URLParam(r, "id")
	// Implementation would fetch from store
	respondJSON(w, http.StatusOK, map[string]string{"id": hackID})
}

// ListContestHacks returns all hacks for a contest
func (h *HackHandler) ListContestHacks(w http.ResponseWriter, r *http.Request) {
	contestID := chi.URLParam(r, "contestId")
	// Implementation would fetch from store
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"contest_id": contestID,
		"hacks":      []model.Hack{},
	})
}
```

- [ ] **Step 2: Add routes**

Add to `internal/api/router.go`:

```go
r.Route("/api/hacks", func(r chi.Router) {
	r.Use(middleware.AuthMiddleware(jwtManager))
	r.Post("/", hackH.SubmitHack)
	r.Get("/{id}", hackH.GetHack)
	r.Get("/contest/{contestId}", hackH.ListContestHacks)
})
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/hack.go internal/api/router.go
git commit -m "feat(hack): add hack API endpoints"
```

---

### Task 6: Frontend Hack UI

**Files:**
- Create: `web/src/pages/HackPanel.tsx`
- Create: `web/src/components/HackResult.tsx`
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Add hack API calls**

Add to `web/src/lib/api.ts`:

```typescript
hacks: {
    submit: (data: { contest_id: string; problem_id: string; submission_id: string; test_input: string }) =>
        request<any>('/hacks', { method: 'POST', body: JSON.stringify(data) }),
    get: (id: string) => request<any>(`/hacks/${id}`),
    listByContest: (contestId: string) => request<any>(`/hacks/contest/${contestId}`),
},
```

- [ ] **Step 2: Create HackResult component**

```tsx
// web/src/components/HackResult.tsx
interface HackResultProps {
  result: {
    hack_id: string;
    status: string;
    success: boolean;
    expected_output?: string;
    actual_output?: string;
    message?: string;
  };
}

export default function HackResult({ result }: HackResultProps) {
  const isSuccess = result.success;
  
  return (
    <div className={`border rounded-lg p-4 ${
      isSuccess ? 'border-green-200 bg-green-50' : 'border-red-200 bg-red-50'
    }`}>
      <h3 className={`font-semibold mb-2 ${
        isSuccess ? 'text-green-800' : 'text-red-800'
      }`}>
        {isSuccess ? '✓ Hack Successful' : '✗ Hack Failed'}
      </h3>
      
      {result.message && (
        <p className="text-sm text-gray-600 mb-2">{result.message}</p>
      )}
      
      {isSuccess && (
        <div className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <p className="text-gray-500 mb-1">Expected Output:</p>
            <pre className="bg-white p-2 rounded border overflow-x-auto">
              {result.expected_output}
            </pre>
          </div>
          <div>
            <p className="text-gray-500 mb-1">Actual Output:</p>
            <pre className="bg-white p-2 rounded border overflow-x-auto">
              {result.actual_output}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Create HackPanel page**

```tsx
// web/src/pages/HackPanel.tsx
import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '../lib/api';
import HackResult from '../components/HackResult';

export default function HackPanel() {
  const { contestId, problemId } = useParams<{ contestId: string; problemId: string }>();
  const [submissions, setSubmissions] = useState<any[]>([]);
  const [selectedSubmission, setSelectedSubmission] = useState('');
  const [testInput, setTestInput] = useState('');
  const [result, setResult] = useState<any>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    // Load hackable submissions
    if (contestId && problemId) {
      // Would need API endpoint to get hackable submissions
    }
  }, [contestId, problemId]);

  const handleSubmit = async () => {
    if (!selectedSubmission || !testInput.trim()) return;
    
    setSubmitting(true);
    try {
      const res = await api.hacks.submit({
        contest_id: contestId!,
        problem_id: problemId!,
        submission_id: selectedSubmission,
        test_input: testInput,
      });
      setResult(res);
    } catch (e: any) {
      alert('Hack failed: ' + e.message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <h1 className="text-2xl font-bold">Hack Challenge</h1>
      
      <div className="bg-white border rounded-lg p-4 space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Select Submission to Hack
          </label>
          <select
            value={selectedSubmission}
            onChange={(e) => setSelectedSubmission(e.target.value)}
            className="w-full border rounded px-3 py-2"
          >
            <option value="">Select a submission...</option>
            {submissions.map((s: any) => (
              <option key={s.id} value={s.id}>
                {s.username} - {s.language} ({s.status})
              </option>
            ))}
          </select>
        </div>
        
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Test Input
          </label>
          <textarea
            value={testInput}
            onChange={(e) => setTestInput(e.target.value)}
            rows={6}
            className="w-full border rounded px-3 py-2 font-mono text-sm"
            placeholder="Enter your test case input..."
          />
        </div>
        
        <button
          onClick={handleSubmit}
          disabled={!selectedSubmission || !testInput.trim() || submitting}
          className="w-full bg-red-600 text-white py-2 rounded hover:bg-red-700 disabled:opacity-50"
        >
          {submitting ? 'Judging...' : 'Submit Hack'}
        </button>
      </div>
      
      {result && <HackResult result={result} />}
    </div>
  );
}
```

- [ ] **Step 4: Add route**

Add to `web/src/App.tsx`:
```tsx
<Route path="/hack/:contestId/:problemId" element={<HackPanel />} />
```

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/HackPanel.tsx web/src/components/HackResult.tsx web/src/lib/api.ts web/src/App.tsx
git commit -m "feat(hack): add hack frontend UI"
```

---

## Verification Checklist

After completing all tasks:

- [ ] Database migration runs successfully
- [ ] Users can submit hacks
- [ ] Hacks are judged correctly
- [ ] Hack results display properly
- [ ] Can't hack own submissions
- [ ] Hack phase timing enforced
- [ ] Hack statistics calculated
- [ ] Rating changes applied after hacks

---

## Notes

1. **Hack phase**: Usually 12 hours after contest ends.
2. **Hack scoring**: +100 for successful hack, -50 for failed hack.
3. **Rate limiting**: Max 10 hacks per hour per user.
4. **Test validation**: Test input must be valid for problem constraints.
