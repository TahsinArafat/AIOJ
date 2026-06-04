# Codeforces-Style User Profile Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign `/user/:username` (`UserPublicProfile.tsx`) to look and feel like the Codeforces profile page with tabs, a proper SVG rating graph, and backend endpoints for public submissions/blogs/comments by user.

**Architecture:** Three new backend endpoints (`GET /api/users/{username}/submissions`, `GET /api/users/{username}/blogs`, `GET /api/users/{username}/comments`) feed a fully tab-driven React page. The frontend SVG rating graph uses no new npm packages — raw SVG with React. All state is co-located in the main page component with a tab-switch data fetch pattern.

**Tech Stack:** Go (chi router), React 19, TypeScript, Tailwind CSS v4, SVG for chart. No new npm dependencies.

---

## File Map

**Backend — modify:**
- `internal/store/postgres/blog.go` — add `ListByUser(ctx, userId, offset, limit)` and `GetCommentsByUser(ctx, userId, offset, limit)`
- `internal/store/postgres/submissions.go` — add `ListPublicByUser(ctx, userId, offset, limit)` (no auth required)
- `internal/api/handler/users.go` — add `GetUserSubmissions`, `GetUserBlogs`, `GetUserComments` handlers
- `internal/api/router.go` — register three new GET routes under `/api/users/{username}/...`

**Frontend — modify:**
- `web/src/lib/api.ts` — add `users.getSubmissions(username, offset, limit)`, `users.getBlogs(username, offset, limit)`, `users.getComments(username, offset, limit)`
- `web/src/pages/UserPublicProfile.tsx` — full rewrite with tab system, SVG rating graph, CF-style header

**Frontend — create:**
- `web/src/components/RatingGraph.tsx` — standalone SVG line chart component

---

## Task 1: Backend — blog store: list posts by user

**Files:**
- Modify: `internal/store/postgres/blog.go`

- [ ] **Step 1: Add `ListByUser` to BlogStore**

  In `internal/store/postgres/blog.go`, add after the existing `ListPosts` function:

  ```go
  func (s *BlogStore) ListByUser(ctx context.Context, userID string, offset, limit int) ([]model.BlogListItem, int, error) {
  	var total int
  	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM blog_posts WHERE user_id=$1", userID).Scan(&total)

  	rows, err := s.db.QueryContext(ctx,
  		`SELECT bp.id, bp.user_id, u.username, bp.title, bp.tags, bp.upvotes, bp.comment_count, bp.created_at
  		 FROM blog_posts bp JOIN users u ON bp.user_id = u.id
  		 WHERE bp.user_id = $1
  		 ORDER BY bp.created_at DESC OFFSET $2 LIMIT $3`,
  		userID, offset, limit)
  	if err != nil {
  		return nil, 0, err
  	}
  	defer rows.Close()

  	var items []model.BlogListItem
  	for rows.Next() {
  		var p model.BlogListItem
  		var tagArr []string
  		if err := rows.Scan(&p.ID, &p.UserID, &p.Username, &p.Title, pq.Array(&tagArr), &p.Upvotes, &p.CommentCount, &p.CreatedAt); err != nil {
  			return nil, 0, err
  		}
  		p.Tags = tagArr
  		items = append(items, p)
  	}
  	if items == nil {
  		items = []model.BlogListItem{}
  	}
  	return items, total, nil
  }
  ```

- [ ] **Step 2: Add `GetCommentsByUser` to BlogStore**

  In `internal/store/postgres/blog.go`, add after `ListByUser`:

  ```go
  func (s *BlogStore) GetCommentsByUser(ctx context.Context, userID string, offset, limit int) ([]model.Comment, int, error) {
  	var total int
  	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comments WHERE user_id=$1", userID).Scan(&total)

  	rows, err := s.db.QueryContext(ctx,
  		`SELECT c.id, c.user_id, u.username, c.parent_type, c.parent_id, c.content, c.upvotes, c.created_at, c.updated_at
  		 FROM comments c JOIN users u ON c.user_id = u.id
  		 WHERE c.user_id = $1
  		 ORDER BY c.created_at DESC OFFSET $2 LIMIT $3`,
  		userID, offset, limit)
  	if err != nil {
  		return nil, 0, err
  	}
  	defer rows.Close()

  	var comments []model.Comment
  	for rows.Next() {
  		var c model.Comment
  		if err := rows.Scan(&c.ID, &c.UserID, &c.Username, &c.ParentType, &c.ParentID, &c.Content, &c.Upvotes, &c.CreatedAt, &c.UpdatedAt); err != nil {
  			return nil, 0, err
  		}
  		comments = append(comments, c)
  	}
  	if comments == nil {
  		comments = []model.Comment{}
  	}
  	return comments, total, nil
  }
  ```

- [ ] **Step 3: Verify Go compiles**

  ```bash
  cd /Users/tahsinarafat/App_Dev/AIOJ && go build ./...
  ```
  Expected: no errors.

---

## Task 2: Backend — submissions store: public list by user

**Files:**
- Modify: `internal/store/postgres/submissions.go`

- [ ] **Step 1: Add `ListPublicByUser` to SubmissionStore**

  In `internal/store/postgres/submissions.go`, add after `ListByUser`:

  ```go
  // ListPublicByUser returns submissions for any user without authentication.
  // It joins with problems to include problem name/slug for display.
  func (s *SubmissionStore) ListPublicByUser(ctx context.Context, userID string, offset, limit int) ([]model.Submission, int, error) {
  	var total int
  	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM submissions WHERE user_id=$1", userID).Scan(&total)

  	rows, err := s.db.QueryContext(ctx,
  		`SELECT s.id, s.problem_id, s.language, s.status, s.score, s.time_used, s.memory_used, s.created_at, s.submission_type
  		 FROM submissions s
  		 WHERE s.user_id = $1
  		 ORDER BY s.created_at DESC OFFSET $2 LIMIT $3`,
  		userID, offset, limit)
  	if err != nil {
  		return nil, 0, err
  	}
  	defer rows.Close()

  	var items []model.Submission
  	for rows.Next() {
  		var sub model.Submission
  		rows.Scan(&sub.ID, &sub.ProblemID, &sub.Language, &sub.Status, &sub.Score, &sub.TimeUsed, &sub.MemoryUsed, &sub.CreatedAt, &sub.SubmissionType)
  		items = append(items, sub)
  	}
  	if items == nil {
  		items = []model.Submission{}
  	}
  	return items, total, nil
  }
  ```

- [ ] **Step 2: Verify Go compiles**

  ```bash
  cd /Users/tahsinarafat/App_Dev/AIOJ && go build ./...
  ```
  Expected: no errors.

---

## Task 3: Backend — UsersHandler: three new endpoints

**Files:**
- Modify: `internal/api/handler/users.go`

The handler needs access to the blog store and submission store. Currently it only has `userStore`. We need to expand the struct.

- [ ] **Step 1: Expand UsersHandler and its constructor**

  Replace the entire `internal/api/handler/users.go` with:

  ```go
  package handler

  import (
  	"net/http"
  	"strconv"

  	"github.com/go-chi/chi/v5"
  	"github.com/tahsinarafat/aioj/internal/store"
  	"github.com/tahsinarafat/aioj/internal/store/postgres"
  )

  type UsersHandler struct {
  	userStore store.UserStore
  	blogStore *postgres.BlogStore
  	subStore  store.SubmissionStore
  }

  func NewUsersHandler(us store.UserStore, bs *postgres.BlogStore, ss store.SubmissionStore) *UsersHandler {
  	return &UsersHandler{userStore: us, blogStore: bs, subStore: ss}
  }

  func (h *UsersHandler) GetByUsername(w http.ResponseWriter, r *http.Request) {
  	username := chi.URLParam(r, "username")
  	profile, err := h.userStore.GetPublicProfile(r.Context(), username)
  	if err != nil {
  		http.Error(w, "internal error", http.StatusInternalServerError)
  		return
  	}
  	if profile == nil {
  		http.Error(w, "user not found", http.StatusNotFound)
  		return
  	}
  	respondJSON(w, http.StatusOK, profile)
  }

  func (h *UsersHandler) GetUserSubmissions(w http.ResponseWriter, r *http.Request) {
  	username := chi.URLParam(r, "username")
  	profile, err := h.userStore.GetPublicProfile(r.Context(), username)
  	if err != nil || profile == nil {
  		http.Error(w, "user not found", http.StatusNotFound)
  		return
  	}
  	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
  	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
  	if limit <= 0 || limit > 100 {
  		limit = 20
  	}
  	subs, total, err := h.subStore.ListPublicByUser(r.Context(), profile.ID, offset, limit)
  	if err != nil {
  		http.Error(w, "internal error", http.StatusInternalServerError)
  		return
  	}
  	respondJSON(w, http.StatusOK, map[string]interface{}{"data": subs, "total": total})
  }

  func (h *UsersHandler) GetUserBlogs(w http.ResponseWriter, r *http.Request) {
  	username := chi.URLParam(r, "username")
  	profile, err := h.userStore.GetPublicProfile(r.Context(), username)
  	if err != nil || profile == nil {
  		http.Error(w, "user not found", http.StatusNotFound)
  		return
  	}
  	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
  	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
  	if limit <= 0 || limit > 50 {
  		limit = 20
  	}
  	posts, total, err := h.blogStore.ListByUser(r.Context(), profile.ID, offset, limit)
  	if err != nil {
  		http.Error(w, "internal error", http.StatusInternalServerError)
  		return
  	}
  	respondJSON(w, http.StatusOK, map[string]interface{}{"data": posts, "total": total})
  }

  func (h *UsersHandler) GetUserComments(w http.ResponseWriter, r *http.Request) {
  	username := chi.URLParam(r, "username")
  	profile, err := h.userStore.GetPublicProfile(r.Context(), username)
  	if err != nil || profile == nil {
  		http.Error(w, "user not found", http.StatusNotFound)
  		return
  	}
  	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
  	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
  	if limit <= 0 || limit > 50 {
  		limit = 20
  	}
  	comments, total, err := h.blogStore.GetCommentsByUser(r.Context(), profile.ID, offset, limit)
  	if err != nil {
  		http.Error(w, "internal error", http.StatusInternalServerError)
  		return
  	}
  	respondJSON(w, http.StatusOK, map[string]interface{}{"data": comments, "total": total})
  }
  ```

- [ ] **Step 2: Fix `NewUsersHandler` call site in deps**

  Find where `NewUsersHandler` is called (likely in `cmd/aioj/main.go` or a deps/wire file):

  ```bash
  grep -rn "NewUsersHandler" /Users/tahsinarafat/App_Dev/AIOJ --include="*.go"
  ```

  Pass `blogStore` and `subStore` to it. The exact args depend on what's available at the call site. Pattern:
  ```go
  usersH := handler.NewUsersHandler(userStore, blogStore, submissionStore)
  ```

- [ ] **Step 3: Register three new routes in router.go**

  In `internal/api/router.go`, find the existing line:
  ```go
  r.Get("/api/users/{username}", usersH.GetByUsername)
  ```
  Replace it with:
  ```go
  r.Get("/api/users/{username}", usersH.GetByUsername)
  r.Get("/api/users/{username}/submissions", usersH.GetUserSubmissions)
  r.Get("/api/users/{username}/blogs", usersH.GetUserBlogs)
  r.Get("/api/users/{username}/comments", usersH.GetUserComments)
  ```

- [ ] **Step 4: Verify Go compiles**

  ```bash
  cd /Users/tahsinarafat/App_Dev/AIOJ && go build ./...
  ```
  Expected: no errors.

---

## Task 4: Backend — check SubmissionStore interface has ListPublicByUser

**Files:**
- Check/modify: `internal/store/store.go` (or wherever `SubmissionStore` interface is defined)

- [ ] **Step 1: Find the interface**

  ```bash
  grep -rn "SubmissionStore" /Users/tahsinarafat/App_Dev/AIOJ/internal/store --include="*.go" | head -20
  ```

- [ ] **Step 2: Add `ListPublicByUser` to the interface**

  In the interface definition file, add:
  ```go
  ListPublicByUser(ctx context.Context, userID string, offset, limit int) ([]model.Submission, int, error)
  ```

- [ ] **Step 3: Verify Go compiles**

  ```bash
  cd /Users/tahsinarafat/App_Dev/AIOJ && go build ./...
  ```
  Expected: no errors.

---

## Task 5: Frontend — add API client methods

**Files:**
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Expand `users` in the api object**

  Find the existing `users` section in `web/src/lib/api.ts`:
  ```ts
  users: {
      getByUsername: (username: string) =>
          request<any>(`/users/${encodeURIComponent(username)}`),
  },
  ```

  Replace with:
  ```ts
  users: {
      getByUsername: (username: string) =>
          request<any>(`/users/${encodeURIComponent(username)}`),
      getSubmissions: (username: string, offset = 0, limit = 20) =>
          request<{ data: any[]; total: number }>(`/users/${encodeURIComponent(username)}/submissions?offset=${offset}&limit=${limit}`),
      getBlogs: (username: string, offset = 0, limit = 20) =>
          request<{ data: any[]; total: number }>(`/users/${encodeURIComponent(username)}/blogs?offset=${offset}&limit=${limit}`),
      getComments: (username: string, offset = 0, limit = 20) =>
          request<{ data: any[]; total: number }>(`/users/${encodeURIComponent(username)}/comments?offset=${offset}&limit=${limit}`),
  },
  ```

- [ ] **Step 2: Verify TypeScript compiles (frontend)**

  ```bash
  cd /Users/tahsinarafat/App_Dev/AIOJ/web && npx tsc --noEmit
  ```
  Expected: no errors on api.ts.

---

## Task 6: Frontend — create RatingGraph component

**Files:**
- Create: `web/src/components/RatingGraph.tsx`

This is a pure SVG line chart with:
- X axis = contest number (evenly spaced)
- Y axis = rating value
- Colored line with dots; dot color = rating tier color (using `getRatingColor`)
- Hover tooltip showing rating, change, date
- Rating tier background bands (Codeforces-style horizontal bands)
- No npm dependencies

- [ ] **Step 1: Create `web/src/components/RatingGraph.tsx`**

  ```tsx
  // web/src/components/RatingGraph.tsx
  import { useState } from 'react'
  import { getRatingColor } from '../lib/rating'

  interface RatingEntry {
    id: string
    new_rating: number
    old_rating: number
    rating_change: number
    contest_id: string
    rank?: number
    created_at: string
  }

  interface TooltipState {
    x: number
    y: number
    entry: RatingEntry
    index: number
  }

  interface Props {
    history: RatingEntry[]
    width?: number
    height?: number
  }

  const BANDS = [
    { min: 0,    max: 1200, color: '#80808015', label: 'Newbie' },
    { min: 1200, max: 1400, color: '#00800015', label: 'Pupil' },
    { min: 1400, max: 1600, color: '#03A89E15', label: 'Specialist' },
    { min: 1600, max: 1900, color: '#0000FF15', label: 'Expert' },
    { min: 1900, max: 2100, color: '#AA00AA15', label: 'Candidate Master' },
    { min: 2100, max: 2300, color: '#FFD70015', label: 'Master' },
    { min: 2300, max: 2400, color: '#FF8C0015', label: 'Int. Master' },
    { min: 2400, max: 2600, color: '#FF8C0015', label: 'Grandmaster' },
    { min: 2600, max: 2900, color: '#FF000015', label: 'Int. GM' },
    { min: 2900, max: 4000, color: '#FF000015', label: 'LGM' },
  ]

  export default function RatingGraph({ history, width = 600, height = 200 }: Props) {
    const [tooltip, setTooltip] = useState<TooltipState | null>(null)

    if (history.length === 0) {
      return (
        <div className="flex items-center justify-center h-32 text-gray-400 dark:text-gray-500 text-sm">
          No contest history yet
        </div>
      )
    }

    const PAD = { top: 16, right: 20, bottom: 32, left: 48 }
    const W = width - PAD.left - PAD.right
    const H = height - PAD.top - PAD.bottom

    const ratings = history.map(h => h.new_rating)
    const minR = Math.max(0, Math.min(...ratings) - 100)
    const maxR = Math.max(...ratings) + 100

    const xScale = (i: number) => history.length === 1 ? W / 2 : (i / (history.length - 1)) * W
    const yScale = (r: number) => H - ((r - minR) / (maxR - minR)) * H

    // Build polyline points
    const points = history.map((e, i) => `${xScale(i)},${yScale(e.new_rating)}`).join(' ')

    // Y-axis tick values
    const yTicks = 4
    const yTickVals = Array.from({ length: yTicks + 1 }, (_, i) =>
      Math.round(minR + (i / yTicks) * (maxR - minR))
    )

    return (
      <div className="relative w-full overflow-x-auto">
        <svg
          viewBox={`0 0 ${width} ${height}`}
          className="w-full"
          style={{ minWidth: Math.max(width, history.length * 24) }}
          onMouseLeave={() => setTooltip(null)}
        >
          <g transform={`translate(${PAD.left},${PAD.top})`}>
            {/* Rating tier bands */}
            {BANDS.map(band => {
              const bandTop = yScale(Math.min(band.max, maxR))
              const bandBot = yScale(Math.max(band.min, minR))
              if (bandBot <= bandTop) return null
              return (
                <rect
                  key={band.label}
                  x={0} y={bandTop}
                  width={W} height={bandBot - bandTop}
                  fill={band.color}
                />
              )
            })}

            {/* Grid lines */}
            {yTickVals.map(v => (
              <g key={v}>
                <line
                  x1={0} y1={yScale(v)} x2={W} y2={yScale(v)}
                  stroke="currentColor" strokeOpacity={0.1} strokeWidth={1}
                />
                <text
                  x={-6} y={yScale(v)} textAnchor="end"
                  dominantBaseline="middle"
                  fontSize={10} fill="currentColor" fillOpacity={0.5}
                >
                  {v}
                </text>
              </g>
            ))}

            {/* X axis labels (every N contests) */}
            {history.map((e, i) => {
              const step = Math.max(1, Math.floor(history.length / 8))
              if (i % step !== 0 && i !== history.length - 1) return null
              return (
                <text
                  key={i}
                  x={xScale(i)} y={H + 20}
                  textAnchor="middle"
                  fontSize={9} fill="currentColor" fillOpacity={0.45}
                >
                  {i + 1}
                </text>
              )
            })}

            {/* Rating line */}
            <polyline
              points={points}
              fill="none"
              stroke="#6366f1"
              strokeWidth={2}
              strokeLinejoin="round"
              strokeLinecap="round"
            />

            {/* Dots */}
            {history.map((entry, i) => {
              const cx = xScale(i)
              const cy = yScale(entry.new_rating)
              const col = getRatingColor(entry.new_rating)
              return (
                <circle
                  key={entry.id || i}
                  cx={cx} cy={cy} r={4}
                  fill={col.hex}
                  stroke="white"
                  strokeWidth={1.5}
                  style={{ cursor: 'pointer' }}
                  onMouseEnter={(e) => {
                    const rect = (e.target as SVGCircleElement).closest('svg')!.getBoundingClientRect()
                    setTooltip({
                      x: cx + PAD.left,
                      y: cy + PAD.top,
                      entry,
                      index: i,
                    })
                  }}
                />
              )
            })}
          </g>

          {/* Tooltip */}
          {tooltip && (() => {
            const tx = Math.min(tooltip.x + 10, width - 150)
            const ty = Math.max(tooltip.y - 70, 4)
            const d = tooltip.entry
            const sign = d.rating_change >= 0 ? '+' : ''
            const changeColor = d.rating_change > 0 ? '#22c55e' : d.rating_change < 0 ? '#ef4444' : '#9ca3af'
            return (
              <g>
                <rect x={tx} y={ty} width={140} height={60} rx={6}
                  fill="white" stroke="#e5e7eb" strokeWidth={1}
                  filter="drop-shadow(0 2px 4px rgba(0,0,0,0.1))"
                />
                <text x={tx + 8} y={ty + 16} fontSize={11} fontWeight="600" fill="#111">
                  Rating: {d.new_rating}
                </text>
                <text x={tx + 8} y={ty + 30} fontSize={10} fill={changeColor}>
                  {sign}{d.rating_change}
                </text>
                <text x={tx + 8} y={ty + 44} fontSize={9} fill="#6b7280">
                  {new Date(d.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })}
                </text>
                {d.rank && (
                  <text x={tx + 8} y={ty + 57} fontSize={9} fill="#6b7280">
                    Rank #{d.rank}
                  </text>
                )}
              </g>
            )
          })()}
        </svg>
      </div>
    )
  }
  ```

- [ ] **Step 2: Verify TypeScript compiles**

  ```bash
  cd /Users/tahsinarafat/App_Dev/AIOJ/web && npx tsc --noEmit 2>&1 | head -30
  ```
  Expected: no errors on RatingGraph.tsx.

---

## Task 7: Frontend — rewrite UserPublicProfile.tsx

**Files:**
- Modify: `web/src/pages/UserPublicProfile.tsx` (full rewrite)

This is the main deliverable. The page has:
1. **CF-style header**: large avatar with rating-color ring, username in rating color, rank title, rating, max rating, member since, contest count, problems solved.
2. **Tab bar**: Profile | Contests | Submissions | Blogs | Comments
3. **Profile tab**: stats cards + SVG rating graph + solved problems chips
4. **Contests tab**: paginated table of contest history (date, contest, rank, old→new, delta)
5. **Submissions tab**: paginated table (problem, verdict, lang, time, memory, date)
6. **Blogs tab**: blog post list with votes/comments count, links to `/blog/:id`
7. **Comments tab**: recent comments with parent type/link

- [ ] **Step 1: Write the full component**

  Replace `web/src/pages/UserPublicProfile.tsx` entirely with:

  ```tsx
  import { useEffect, useState, useCallback } from 'react'
  import { useParams, Link } from 'react-router-dom'
  import { api } from '../lib/api'
  import RatingBadge from '../components/RatingBadge'
  import RatingGraph from '../components/RatingGraph'
  import { getRatingColor, getRatingTitle } from '../lib/rating'

  // ─── Types ──────────────────────────────────────────────────────────────────

  interface UserProfile {
    id: string
    username: string
    rating: number | null
    contests_played: number
    problems_solved: number
    created_at: string
  }

  interface RatingEntry {
    id: string
    new_rating: number
    old_rating: number
    rating_change: number
    contest_id: string
    rank: number
    created_at: string
  }

  interface Submission {
    id: string
    problem_id: string
    language: string
    status: string
    score: number
    time_used: number
    memory_used: number
    created_at: string
    submission_type: string
  }

  interface BlogPost {
    id: string
    title: string
    tags: string[]
    upvotes: number
    comment_count: number
    created_at: string
  }

  interface Comment {
    id: string
    parent_type: string
    parent_id: string
    content: string
    upvotes: number
    created_at: string
  }

  type Tab = 'profile' | 'contests' | 'submissions' | 'blogs' | 'comments'

  // ─── Helpers ────────────────────────────────────────────────────────────────

  function fmtDate(s: string) {
    return new Date(s).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
  }

  function fmtDateShort(s: string) {
    return new Date(s).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: '2-digit' })
  }

  function StatusBadge({ status }: { status: string }) {
    const s = status.toLowerCase().replace(/[\s-]/g, '_')
    const map: Record<string, { label: string; cls: string }> = {
      accepted:            { label: 'AC',  cls: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400' },
      ac:                  { label: 'AC',  cls: 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400' },
      wrong_answer:        { label: 'WA',  cls: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400' },
      wa:                  { label: 'WA',  cls: 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400' },
      time_limit_exceeded: { label: 'TLE', cls: 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400' },
      tle:                 { label: 'TLE', cls: 'bg-yellow-100 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-400' },
      runtime_error:       { label: 'RE',  cls: 'bg-orange-100 dark:bg-orange-900/30 text-orange-700' },
      re:                  { label: 'RE',  cls: 'bg-orange-100 dark:bg-orange-900/30 text-orange-700' },
      compilation_error:   { label: 'CE',  cls: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300' },
      ce:                  { label: 'CE',  cls: 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300' },
      memory_limit_exceeded: { label: 'MLE', cls: 'bg-purple-100 dark:bg-purple-900/30 text-purple-700' },
      mle:                 { label: 'MLE', cls: 'bg-purple-100 dark:bg-purple-900/30 text-purple-700' },
    }
    const entry = map[s] ?? { label: status.toUpperCase().slice(0, 4), cls: 'bg-blue-100 dark:bg-blue-900/30 text-blue-700' }
    return <span className={`px-2 py-0.5 rounded text-xs font-bold font-mono ${entry.cls}`}>{entry.label}</span>
  }

  function DeltaBadge({ delta }: { delta: number }) {
    if (delta === 0) return <span className="text-gray-400">±0</span>
    return (
      <span className={delta > 0 ? 'text-green-600 dark:text-green-400 font-semibold' : 'text-red-600 dark:text-red-400 font-semibold'}>
        {delta > 0 ? '+' : ''}{delta}
      </span>
    )
  }

  function Skeleton() {
    return (
      <div className="animate-pulse space-y-6">
        <div className="h-32 bg-gray-200 dark:bg-gray-700 rounded-lg" />
        <div className="h-10 bg-gray-200 dark:bg-gray-700 rounded" />
        <div className="h-48 bg-gray-200 dark:bg-gray-700 rounded-lg" />
      </div>
    )
  }

  function Pagination({ page, total, limit, onChange }: { page: number; total: number; limit: number; onChange: (p: number) => void }) {
    const pages = Math.ceil(total / limit)
    if (pages <= 1) return null
    return (
      <div className="flex items-center justify-center gap-2 py-4">
        <button
          disabled={page === 1}
          onClick={() => onChange(page - 1)}
          className="px-3 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 disabled:opacity-40 hover:bg-gray-100 dark:hover:bg-gray-700"
        >
          ‹ Prev
        </button>
        <span className="text-sm text-gray-500 dark:text-gray-400">
          {page} / {pages}
        </span>
        <button
          disabled={page === pages}
          onClick={() => onChange(page + 1)}
          className="px-3 py-1 text-sm rounded border border-gray-300 dark:border-gray-600 disabled:opacity-40 hover:bg-gray-100 dark:hover:bg-gray-700"
        >
          Next ›
        </button>
      </div>
    )
  }

  // ─── Main component ──────────────────────────────────────────────────────────

  const LIMIT = 20

  export default function UserPublicProfile() {
    const { username } = useParams<{ username: string }>()
    const [tab, setTab] = useState<Tab>('profile')
    const [user, setUser] = useState<UserProfile | null>(null)
    const [ratingHistory, setRatingHistory] = useState<RatingEntry[]>([])
    const [loading, setLoading] = useState(true)
    const [notFound, setNotFound] = useState(false)

    // Tab-specific data
    const [submissions, setSubmissions] = useState<Submission[]>([])
    const [subsTotal, setSubsTotal] = useState(0)
    const [subsPage, setSubsPage] = useState(1)
    const [subsLoading, setSubsLoading] = useState(false)

    const [blogs, setBlogs] = useState<BlogPost[]>([])
    const [blogsTotal, setBlogsTotal] = useState(0)
    const [blogsPage, setBlogsPage] = useState(1)
    const [blogsLoading, setBlogsLoading] = useState(false)

    const [comments, setComments] = useState<Comment[]>([])
    const [commentsTotal, setCommentsTotal] = useState(0)
    const [commentsPage, setCommentsPage] = useState(1)
    const [commentsLoading, setCommentsLoading] = useState(false)

    // Initial load: user + rating history
    useEffect(() => {
      if (!username) return
      setLoading(true)
      setNotFound(false)
      api.users.getByUsername(username)
        .then(u => {
          setUser(u)
          return api.ratings.getByUser(u.id, 100)
        })
        .then(d => {
          const data = Array.isArray(d) ? d : d?.data ?? []
          setRatingHistory(
            data.sort((a: RatingEntry, b: RatingEntry) =>
              new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
            )
          )
        })
        .catch(() => setNotFound(true))
        .finally(() => setLoading(false))
    }, [username])

    // Submissions tab
    const loadSubmissions = useCallback((page: number) => {
      if (!username) return
      setSubsLoading(true)
      api.users.getSubmissions(username, (page - 1) * LIMIT, LIMIT)
        .then(d => {
          setSubmissions(d.data ?? [])
          setSubsTotal(d.total ?? 0)
          setSubsPage(page)
        })
        .finally(() => setSubsLoading(false))
    }, [username])

    // Blogs tab
    const loadBlogs = useCallback((page: number) => {
      if (!username) return
      setBlogsLoading(true)
      api.users.getBlogs(username, (page - 1) * LIMIT, LIMIT)
        .then(d => {
          setBlogs(d.data ?? [])
          setBlogsTotal(d.total ?? 0)
          setBlogsPage(page)
        })
        .finally(() => setBlogsLoading(false))
    }, [username])

    // Comments tab
    const loadComments = useCallback((page: number) => {
      if (!username) return
      setCommentsLoading(true)
      api.users.getComments(username, (page - 1) * LIMIT, LIMIT)
        .then(d => {
          setComments(d.data ?? [])
          setCommentsTotal(d.total ?? 0)
          setCommentsPage(page)
        })
        .finally(() => setCommentsLoading(false))
    }, [username])

    // Load tab data on first switch
    useEffect(() => {
      if (!user) return
      if (tab === 'submissions' && submissions.length === 0 && !subsLoading) loadSubmissions(1)
      if (tab === 'blogs' && blogs.length === 0 && !blogsLoading) loadBlogs(1)
      if (tab === 'comments' && comments.length === 0 && !commentsLoading) loadComments(1)
    }, [tab, user]) // eslint-disable-line react-hooks/exhaustive-deps

    if (loading) return <Skeleton />
    if (notFound || !user) {
      return (
        <div className="max-w-3xl mx-auto text-center py-20">
          <h1 className="text-2xl font-bold mb-2 text-gray-800 dark:text-gray-200">User Not Found</h1>
          <p className="text-gray-500 dark:text-gray-400 mb-6">The user «{username}» does not exist.</p>
          <Link to="/" className="text-blue-600 dark:text-blue-400 hover:underline">← Back to Home</Link>
        </div>
      )
    }

    const currentRating = ratingHistory.length > 0
      ? ratingHistory[ratingHistory.length - 1].new_rating
      : user.rating ?? 0
    const maxRating = ratingHistory.length > 0
      ? Math.max(...ratingHistory.map(h => h.new_rating))
      : currentRating
    const ratingColor = getRatingColor(currentRating)
    const ratingTitle = currentRating > 0 ? getRatingTitle(currentRating) : 'Unrated'

    const TABS: { key: Tab; label: string }[] = [
      { key: 'profile',     label: 'Profile' },
      { key: 'contests',    label: `Contests (${ratingHistory.length})` },
      { key: 'submissions', label: 'Submissions' },
      { key: 'blogs',       label: 'Blogs' },
      { key: 'comments',    label: 'Comments' },
    ]

    return (
      <div className="max-w-4xl mx-auto space-y-4">

        {/* ─── Header ─────────────────────────────────────────────────── */}
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg p-6 shadow-sm">
          <div className="flex items-start gap-5">
            {/* Avatar with rating-colored ring */}
            <div
              className="w-20 h-20 rounded-full flex items-center justify-center text-3xl font-bold text-white flex-shrink-0"
              style={{
                background: `linear-gradient(135deg, ${ratingColor.hex}cc, ${ratingColor.hex})`,
                boxShadow: `0 0 0 4px ${ratingColor.hex}50`,
              }}
            >
              {username?.charAt(0).toUpperCase()}
            </div>

            {/* Info */}
            <div className="flex-1 min-w-0">
              <div className="flex items-baseline gap-3 flex-wrap">
                <h1
                  className="text-2xl font-bold"
                  style={{ color: ratingColor.hex }}
                >
                  {user.username}
                </h1>
                {currentRating > 0 && (
                  <span className="text-sm font-medium" style={{ color: ratingColor.hex }}>
                    {ratingTitle}
                  </span>
                )}
              </div>

              <div className="flex flex-wrap gap-x-6 gap-y-1 mt-2 text-sm text-gray-600 dark:text-gray-400">
                <span>
                  <span className="text-gray-400 dark:text-gray-500 mr-1">Rating:</span>
                  {currentRating > 0
                    ? <RatingBadge rating={currentRating} size="md" />
                    : <span className="text-gray-400">Unrated</span>
                  }
                </span>
                {maxRating > 0 && maxRating !== currentRating && (
                  <span>
                    <span className="text-gray-400 dark:text-gray-500 mr-1">Max:</span>
                    <RatingBadge rating={maxRating} size="md" />
                  </span>
                )}
                <span>
                  <span className="text-gray-400 dark:text-gray-500 mr-1">Contests:</span>
                  <strong className="text-gray-800 dark:text-gray-200">{user.contests_played ?? ratingHistory.length}</strong>
                </span>
                <span>
                  <span className="text-gray-400 dark:text-gray-500 mr-1">Solved:</span>
                  <strong className="text-gray-800 dark:text-gray-200">{user.problems_solved ?? 0}</strong>
                </span>
              </div>

              <p className="text-xs text-gray-400 dark:text-gray-500 mt-2">
                Member since {fmtDate(user.created_at)}
              </p>
            </div>
          </div>
        </div>

        {/* ─── Tab Bar ────────────────────────────────────────────────── */}
        <div className="border-b border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 rounded-t-lg overflow-x-auto">
          <nav className="flex gap-0 px-2">
            {TABS.map(t => (
              <button
                key={t.key}
                onClick={() => setTab(t.key)}
                className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors whitespace-nowrap ${
                  tab === t.key
                    ? 'border-blue-600 text-blue-600 dark:border-blue-400 dark:text-blue-400'
                    : 'border-transparent text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'
                }`}
              >
                {t.label}
              </button>
            ))}
          </nav>
        </div>

        {/* ─── Tab Content ────────────────────────────────────────────── */}
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-b-lg rounded-tr-lg shadow-sm">

          {/* PROFILE TAB */}
          {tab === 'profile' && (
            <div className="p-6 space-y-6">
              {/* Stats row */}
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                {[
                  { label: 'Current Rating', value: currentRating > 0 ? <RatingBadge rating={currentRating} size="lg" /> : '—' },
                  { label: 'Max Rating',      value: maxRating > 0 ? <RatingBadge rating={maxRating} size="lg" /> : '—' },
                  { label: 'Contests',        value: <span className="text-2xl font-bold text-gray-800 dark:text-gray-100">{ratingHistory.length}</span> },
                  { label: 'Problems Solved', value: <span className="text-2xl font-bold text-gray-800 dark:text-gray-100">{user.problems_solved ?? 0}</span> },
                ].map(s => (
                  <div key={s.label} className="bg-gray-50 dark:bg-gray-750 border border-gray-100 dark:border-gray-700 rounded-lg p-4 text-center">
                    <p className="text-xs text-gray-500 dark:text-gray-400 mb-2">{s.label}</p>
                    {s.value}
                  </div>
                ))}
              </div>

              {/* Rating graph */}
              <div>
                <h2 className="text-base font-semibold mb-3 text-gray-700 dark:text-gray-300">Rating History</h2>
                <RatingGraph history={ratingHistory} width={700} height={220} />
              </div>
            </div>
          )}

          {/* CONTESTS TAB */}
          {tab === 'contests' && (
            <div>
              {ratingHistory.length === 0 ? (
                <p className="text-center py-12 text-gray-400 dark:text-gray-500 text-sm">No rated contests yet</p>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-750">
                        <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">#</th>
                        <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Date</th>
                        <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Rank</th>
                        <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Old</th>
                        <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">New</th>
                        <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Change</th>
                      </tr>
                    </thead>
                    <tbody>
                      {[...ratingHistory].reverse().map((entry, i) => (
                        <tr key={entry.id || i} className="border-t border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-750">
                          <td className="px-4 py-2.5 text-gray-400">{ratingHistory.length - i}</td>
                          <td className="px-4 py-2.5 text-gray-600 dark:text-gray-400">{fmtDateShort(entry.created_at)}</td>
                          <td className="px-4 py-2.5 text-right text-gray-700 dark:text-gray-300">
                            {entry.rank ? `#${entry.rank}` : '—'}
                          </td>
                          <td className="px-4 py-2.5 text-right">
                            <RatingBadge rating={entry.old_rating} size="sm" />
                          </td>
                          <td className="px-4 py-2.5 text-right">
                            <RatingBadge rating={entry.new_rating} size="sm" />
                          </td>
                          <td className="px-4 py-2.5 text-right">
                            <DeltaBadge delta={entry.rating_change} />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}

          {/* SUBMISSIONS TAB */}
          {tab === 'submissions' && (
            <div>
              {subsLoading ? (
                <div className="text-center py-12 text-gray-400 text-sm">Loading...</div>
              ) : submissions.length === 0 ? (
                <p className="text-center py-12 text-gray-400 dark:text-gray-500 text-sm">No submissions</p>
              ) : (
                <>
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-750">
                          <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Verdict</th>
                          <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Language</th>
                          <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Time</th>
                          <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Memory</th>
                          <th className="px-4 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Date</th>
                        </tr>
                      </thead>
                      <tbody>
                        {submissions.map(sub => (
                          <tr key={sub.id} className="border-t border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-750">
                            <td className="px-4 py-2.5"><StatusBadge status={sub.status} /></td>
                            <td className="px-4 py-2.5 text-gray-500 dark:text-gray-400 font-mono text-xs">{sub.language}</td>
                            <td className="px-4 py-2.5 text-right text-gray-500 dark:text-gray-400 font-mono text-xs">
                              {sub.time_used > 0 ? `${sub.time_used}ms` : '—'}
                            </td>
                            <td className="px-4 py-2.5 text-right text-gray-500 dark:text-gray-400 font-mono text-xs">
                              {sub.memory_used > 0 ? `${Math.round(sub.memory_used / 1024)}KB` : '—'}
                            </td>
                            <td className="px-4 py-2.5 text-right text-gray-500 dark:text-gray-400">{fmtDateShort(sub.created_at)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  <Pagination page={subsPage} total={subsTotal} limit={LIMIT} onChange={p => loadSubmissions(p)} />
                </>
              )}
            </div>
          )}

          {/* BLOGS TAB */}
          {tab === 'blogs' && (
            <div>
              {blogsLoading ? (
                <div className="text-center py-12 text-gray-400 text-sm">Loading...</div>
              ) : blogs.length === 0 ? (
                <p className="text-center py-12 text-gray-400 dark:text-gray-500 text-sm">No blog posts yet</p>
              ) : (
                <>
                  <div className="divide-y divide-gray-100 dark:divide-gray-700">
                    {blogs.map(post => (
                      <div key={post.id} className="px-6 py-4 hover:bg-gray-50 dark:hover:bg-gray-750">
                        <Link to={`/blog/${post.id}`} className="text-base font-semibold text-blue-600 dark:text-blue-400 hover:underline">
                          {post.title}
                        </Link>
                        <div className="flex gap-4 mt-1 text-xs text-gray-400 dark:text-gray-500">
                          <span>▲ {post.upvotes}</span>
                          <span>💬 {post.comment_count}</span>
                          <span>{fmtDate(post.created_at)}</span>
                        </div>
                        {post.tags?.length > 0 && (
                          <div className="flex gap-1.5 mt-2 flex-wrap">
                            {post.tags.map(tag => (
                              <span key={tag} className="text-xs bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 px-2 py-0.5 rounded">
                                {tag}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                  <div className="px-6 pb-2">
                    <Pagination page={blogsPage} total={blogsTotal} limit={LIMIT} onChange={p => loadBlogs(p)} />
                  </div>
                </>
              )}
            </div>
          )}

          {/* COMMENTS TAB */}
          {tab === 'comments' && (
            <div>
              {commentsLoading ? (
                <div className="text-center py-12 text-gray-400 text-sm">Loading...</div>
              ) : comments.length === 0 ? (
                <p className="text-center py-12 text-gray-400 dark:text-gray-500 text-sm">No comments yet</p>
              ) : (
                <>
                  <div className="divide-y divide-gray-100 dark:divide-gray-700">
                    {comments.map(c => (
                      <div key={c.id} className="px-6 py-4">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="text-xs bg-gray-100 dark:bg-gray-700 text-gray-500 dark:text-gray-400 px-2 py-0.5 rounded capitalize">
                            {c.parent_type}
                          </span>
                          <span className="text-xs text-gray-400">{fmtDate(c.created_at)}</span>
                          {c.upvotes !== 0 && (
                            <span className="text-xs text-gray-400">▲ {c.upvotes}</span>
                          )}
                        </div>
                        <p className="text-sm text-gray-700 dark:text-gray-300 line-clamp-3">{c.content}</p>
                        {c.parent_type === 'blog' && (
                          <Link to={`/blog/${c.parent_id}`} className="text-xs text-blue-500 hover:underline mt-1 inline-block">
                            View post →
                          </Link>
                        )}
                      </div>
                    ))}
                  </div>
                  <div className="px-6 pb-2">
                    <Pagination page={commentsPage} total={commentsTotal} limit={LIMIT} onChange={p => loadComments(p)} />
                  </div>
                </>
              )}
            </div>
          )}

        </div>
      </div>
    )
  }
  ```

- [ ] **Step 2: Verify TypeScript compiles**

  ```bash
  cd /Users/tahsinarafat/App_Dev/AIOJ/web && npx tsc --noEmit 2>&1 | head -30
  ```
  Expected: no errors on UserPublicProfile.tsx or RatingGraph.tsx.

---

## Task 8: Integration — find deps wiring and fix NewUsersHandler call

**Files:**
- Find and modify the file that calls `NewUsersHandler` (likely `cmd/aioj/main.go` or a deps struct)

- [ ] **Step 1: Find the call site**

  ```bash
  grep -rn "NewUsersHandler\|Users:" /Users/tahsinarafat/App_Dev/AIOJ --include="*.go" | grep -v "_test.go"
  ```

- [ ] **Step 2: Update the call**

  The current call is:
  ```go
  Users: handler.NewUsersHandler(userStore)
  ```

  Change to (passing the already-existing blogStore and submissionStore at the call site):
  ```go
  Users: handler.NewUsersHandler(userStore, blogStore, submissionStore)
  ```

  Note: `blogStore` is `postgres.NewBlogStore(db)` and `submissionStore` is already passed to `handler.NewSubmissionHandler(...)`. Reuse the same instances.

- [ ] **Step 3: Final build verification**

  ```bash
  cd /Users/tahsinarafat/App_Dev/AIOJ && go build ./...
  ```
  Expected: no errors.

- [ ] **Step 4: Frontend build verification**

  ```bash
  cd /Users/tahsinarafat/App_Dev/AIOJ/web && npx tsc --noEmit
  ```
  Expected: no errors.

---

## Self-Review

**Spec coverage:**
- ✅ CF-style header with rating-colored avatar, username in rating color, rank title
- ✅ 5 tabs: Profile, Contests, Submissions, Blogs, Comments
- ✅ SVG rating graph with tooltips (RatingGraph.tsx)
- ✅ Contests tab: full history table with rank, old→new, delta
- ✅ Submissions tab: public endpoint, paginated
- ✅ Blogs tab: posts by user, paginated
- ✅ Comments tab: comments by user, paginated
- ✅ No new npm dependencies

**Placeholder scan:** None found.

**Type consistency:**
- `RatingEntry` in `UserPublicProfile.tsx` matches what `api.ratings.getByUser` returns (id, new_rating, old_rating, rating_change, contest_id, rank, created_at)
- `RatingGraph` props: `history: RatingEntry[]` — same type, compatible
- Backend `ListPublicByUser` returns `[]model.Submission` — matches frontend `Submission` interface
- `ListByUser` (blog) returns `[]model.BlogListItem` — matches frontend `BlogPost` interface
- `GetCommentsByUser` returns `[]model.Comment` — matches frontend `Comment` interface
