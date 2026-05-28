# Sub-Plan 10: Problem Statistics

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show detailed problem statistics including language distribution, solver stats, and difficulty estimation.

**Architecture:** Add statistics queries, create statistics API, frontend statistics display.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Create
- `internal/api/handler/stats.go` - Statistics handler

### Backend Files to Modify
- `internal/store/postgres/submissions.go` - Add statistics queries
- `internal/api/router.go` - Add stats routes

### Frontend Files to Create
- `web/src/components/ProblemStats.tsx` - Statistics component
- `web/src/components/LanguageChart.tsx` - Language distribution chart

### Frontend Files to Modify
- `web/src/pages/ProblemDetail.tsx` - Add statistics tab

---

## Tasks

### Task 1: Backend Statistics Queries

**Files:**
- Modify: `internal/store/postgres/submissions.go`

- [ ] **Step 1: Add statistics methods**

```go
type ProblemStats struct {
	TotalSubmissions   int                `json:"total_submissions"`
	AcceptedSubmissions int               `json:"accepted_submissions"`
	AcceptanceRate     float64            `json:"acceptance_rate"`
	UniqueSolvers      int                `json:"unique_solvers"`
	AverageAttempts    float64            `json:"average_attempts"`
	LanguageDistribution map[string]int   `json:"language_distribution"`
	DifficultyEstimate float64            `json:"difficulty_estimate"`
}

func (s *SubmissionStore) GetProblemStats(ctx context.Context, problemID string) (*ProblemStats, error) {
	stats := &ProblemStats{
		LanguageDistribution: make(map[string]int),
	}
	
	// Total submissions
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM submissions WHERE problem_id = $1",
		problemID).Scan(&stats.TotalSubmissions)
	
	// Accepted submissions
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM submissions WHERE problem_id = $1 AND status = 'ac'",
		problemID).Scan(&stats.AcceptedSubmissions)
	
	// Acceptance rate
	if stats.TotalSubmissions > 0 {
		stats.AcceptanceRate = float64(stats.AcceptedSubmissions) / float64(stats.TotalSubmissions) * 100
	}
	
	// Unique solvers
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(DISTINCT user_id) FROM submissions WHERE problem_id = $1 AND status = 'ac'",
		problemID).Scan(&stats.UniqueSolvers)
	
	// Language distribution
	rows, err := s.db.QueryContext(ctx,
		`SELECT language, COUNT(*) FROM submissions 
		 WHERE problem_id = $1 GROUP BY language ORDER BY COUNT(*) DESC`,
		problemID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var lang string
			var count int
			rows.Scan(&lang, &count)
			stats.LanguageDistribution[lang] = count
		}
	}
	
	// Average attempts for solvers
	s.db.QueryRowContext(ctx,
		`SELECT AVG(attempts) FROM (
			SELECT user_id, COUNT(*) as attempts 
			FROM submissions WHERE problem_id = $1 
			GROUP BY user_id
			HAVING COUNT(CASE WHEN status = 'ac' THEN 1 END) > 0
		) t`,
		problemID).Scan(&stats.AverageAttempts)
	
	return stats, nil
}

type UserProblemStats struct {
	ProblemsSolved    int     `json:"problems_solved"`
	TotalSubmissions  int     `json:"total_submissions"`
	AcceptanceRate    float64 `json:"acceptance_rate"`
	FavoriteLanguage  string  `json:"favorite_language"`
	StreakDays        int     `json:"streak_days"`
}

func (s *SubmissionStore) GetUserStats(ctx context.Context, userID string) (*UserProblemStats, error) {
	stats := &UserProblemStats{}
	
	// Problems solved
	s.db.QueryRowContext(ctx,
		"SELECT problems_solved FROM user_profiles WHERE user_id = $1",
		userID).Scan(&stats.ProblemsSolved)
	
	// Total submissions
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM submissions WHERE user_id = $1",
		userID).Scan(&stats.TotalSubmissions)
	
	// Acceptance rate
	var accepted int
	s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM submissions WHERE user_id = $1 AND status = 'ac'",
		userID).Scan(&accepted)
	if stats.TotalSubmissions > 0 {
		stats.AcceptanceRate = float64(accepted) / float64(stats.TotalSubmissions) * 100
	}
	
	// Favorite language
	s.db.QueryRowContext(ctx,
		`SELECT language FROM submissions WHERE user_id = $1 
		 GROUP BY language ORDER BY COUNT(*) DESC LIMIT 1`,
		userID).Scan(&stats.FavoriteLanguage)
	
	return stats, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/store/postgres/submissions.go
git commit -m "feat(stats): add statistics queries"
```

---

### Task 2: Statistics Handler

**Files:**
- Create: `internal/api/handler/stats.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create statistics handler**

```go
// internal/api/handler/stats.go
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type StatsHandler struct {
	submissionStore *postgres.SubmissionStore
}

func NewStatsHandler(ss *postgres.SubmissionStore) *StatsHandler {
	return &StatsHandler{submissionStore: ss}
}

func (h *StatsHandler) GetProblemStats(w http.ResponseWriter, r *http.Request) {
	problemID := chi.URLParam(r, "problemId")
	stats, err := h.submissionStore.GetProblemStats(r.Context(), problemID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

func (h *StatsHandler) GetUserStats(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	stats, err := h.submissionStore.GetUserStats(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, stats)
}
```

- [ ] **Step 2: Add routes**

```go
r.Route("/api/stats", func(r chi.Router) {
	r.Get("/problems/{problemId}", statsH.GetProblemStats)
	r.Get("/me", statsH.GetUserStats)
})
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/stats.go internal/api/router.go
git commit -m "feat(stats): add statistics API endpoints"
```

---

### Task 3: Frontend Statistics Components

**Files:**
- Create: `web/src/components/ProblemStats.tsx`
- Create: `web/src/components/LanguageChart.tsx`
- Modify: `web/src/pages/ProblemDetail.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add stats API calls**

```typescript
stats: {
    getProblemStats: (problemId: string) => request<any>(`/stats/problems/${problemId}`),
    getMyStats: () => request<any>('/stats/me'),
},
```

- [ ] **Step 2: Create LanguageChart component**

```tsx
// web/src/components/LanguageChart.tsx
interface LanguageChartProps {
  distribution: Record<string, number>;
}

export default function LanguageChart({ distribution }: LanguageChartProps) {
  const total = Object.values(distribution).reduce((a, b) => a + b, 0);
  if (total === 0) return <p className="text-gray-500">No data</p>;
  
  const sorted = Object.entries(distribution)
    .sort(([, a], [, b]) => b - a)
    .slice(0, 8);
  
  const colors = [
    'bg-blue-500', 'bg-green-500', 'bg-yellow-500', 'bg-red-500',
    'bg-purple-500', 'bg-pink-500', 'bg-indigo-500', 'bg-gray-500',
  ];
  
  return (
    <div className="space-y-2">
      {sorted.map(([lang, count], i) => (
        <div key={lang} className="flex items-center gap-2">
          <span className="w-20 text-sm text-gray-600 truncate">{lang}</span>
          <div className="flex-1 bg-gray-200 rounded-full h-4">
            <div
              className={`${colors[i]} h-4 rounded-full`}
              style={{ width: `${(count / total) * 100}%` }}
            />
          </div>
          <span className="w-12 text-sm text-gray-500 text-right">
            {Math.round((count / total) * 100)}%
          </span>
        </div>
      ))}
    </div>
  );
}
```

- [ ] **Step 3: Create ProblemStats component**

```tsx
// web/src/components/ProblemStats.tsx
import { useEffect, useState } from 'react';
import { api } from '../lib/api';
import LanguageChart from './LanguageChart';

interface ProblemStatsProps {
  problemId: string;
}

export default function ProblemStats({ problemId }: ProblemStatsProps) {
  const [stats, setStats] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.stats.getProblemStats(problemId)
      .then(setStats)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [problemId]);

  if (loading) return <div>Loading stats...</div>;
  if (!stats) return null;

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-gray-50 p-4 rounded-lg text-center">
          <p className="text-2xl font-bold">{stats.total_submissions}</p>
          <p className="text-sm text-gray-500">Submissions</p>
        </div>
        <div className="bg-gray-50 p-4 rounded-lg text-center">
          <p className="text-2xl font-bold">{stats.unique_solvers}</p>
          <p className="text-sm text-gray-500">Solvers</p>
        </div>
        <div className="bg-gray-50 p-4 rounded-lg text-center">
          <p className="text-2xl font-bold">{stats.acceptance_rate.toFixed(1)}%</p>
          <p className="text-sm text-gray-500">Acceptance</p>
        </div>
        <div className="bg-gray-50 p-4 rounded-lg text-center">
          <p className="text-2xl font-bold">{stats.average_attempts.toFixed(1)}</p>
          <p className="text-sm text-gray-500">Avg Attempts</p>
        </div>
      </div>

      <div>
        <h3 className="font-semibold mb-3">Language Distribution</h3>
        <LanguageChart distribution={stats.language_distribution} />
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Add stats tab to ProblemDetail**

```tsx
// Add to ProblemDetail
import ProblemStats from '../components/ProblemStats';

// Add tab state
const [activeTab, setActiveTab] = useState<'statement' | 'stats'>('statement');

// Add tab buttons
<div className="flex gap-4 mb-4">
  <button
    onClick={() => setActiveTab('statement')}
    className={activeTab === 'statement' ? 'font-semibold border-b-2 border-blue-600' : 'text-gray-500'}
  >
    Statement
  </button>
  <button
    onClick={() => setActiveTab('stats')}
    className={activeTab === 'stats' ? 'font-semibold border-b-2 border-blue-600' : 'text-gray-500'}
  >
    Statistics
  </button>
</div>

// Add stats content
{activeTab === 'stats' && <ProblemStats problemId={problem.id} />}
```

- [ ] **Step 5: Commit**

```bash
git add web/src/components/ProblemStats.tsx web/src/components/LanguageChart.tsx web/src/pages/ProblemDetail.tsx web/src/lib/api.ts
git commit -m "feat(stats): add problem statistics UI"
```

---

## Verification Checklist

- [ ] Total submissions count correct
- [ ] Unique solvers count correct
- [ ] Acceptance rate calculated correctly
- [ ] Language distribution displays
- [ ] Average attempts shown
- [ ] Stats tab works on problem detail

---

## Notes

1. **Acceptance rate**: accepted / total * 100
2. **Unique solvers**: Distinct users with AC
3. **Average attempts**: Average submissions per solver
4. **Language distribution**: Top 8 languages shown
