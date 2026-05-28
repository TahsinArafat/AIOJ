# Sub-Plan 12: Groups

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to create and join groups for organized training, team contests, and discussions.

**Architecture:** Add `groups` and `group_members` tables, group service, frontend group management UI.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Create
- `internal/model/group.go` - Group models
- `internal/store/postgres/groups.go` - Group store
- `internal/api/handler/group.go` - Group handler

### Backend Files to Modify
- `internal/store/interfaces.go` - Add GroupStore interface
- `internal/api/router.go` - Add group routes

### Frontend Files to Create
- `web/src/pages/GroupList.tsx` - Group listing
- `web/src/pages/GroupDetail.tsx` - Group detail page
- `web/src/pages/GroupCreate.tsx` - Create group form

### Frontend Files to Modify
- `web/src/App.tsx` - Add group routes
- `web/src/lib/api.ts` - Add group API calls

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000007_groups.up.sql`
- Create: `internal/store/migrations/000007_groups.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000007_groups.up.sql

CREATE TABLE groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_public BOOLEAN NOT NULL DEFAULT true,
    max_members INTEGER,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE group_members (
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(16) NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE group_contests (
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, contest_id)
);

CREATE INDEX idx_group_members_user ON group_members(user_id);
CREATE INDEX idx_groups_public ON groups(is_public) WHERE is_public = true;
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000007_groups.down.sql

DROP TABLE IF EXISTS group_contests;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`
Expected: Migration applied successfully

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000007_groups.*
git commit -m "feat(groups): add groups database migration"
```

---

### Task 2: Group Models

**Files:**
- Create: `internal/model/group.go`

- [ ] **Step 1: Create group models**

```go
// internal/model/group.go
package model

import "time"

type Group struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	IsPublic    bool       `json:"is_public"`
	MaxMembers  *int       `json:"max_members,omitempty"`
	CreatedBy   string     `json:"created_by"`
	CreatorName string     `json:"creator_name,omitempty"`
	MemberCount int        `json:"member_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type GroupMember struct {
	GroupID    string    `json:"group_id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username,omitempty"`
	Role       string    `json:"role"`
	JoinedAt   time.Time `json:"joined_at"`
}

type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	MaxMembers  *int   `json:"max_members,omitempty"`
}

type GroupListItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/group.go
git commit -m "feat(groups): add group models"
```

---

### Task 3: Group Store

**Files:**
- Create: `internal/store/postgres/groups.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Add GroupStore interface**

Add to `internal/store/interfaces.go`:

```go
type GroupStore interface {
	Create(ctx context.Context, g *model.Group) error
	GetByID(ctx context.Context, id string) (*model.Group, error)
	List(ctx context.Context, offset, limit int) ([]model.GroupListItem, int, error)
	ListByUser(ctx context.Context, userID string) ([]model.GroupListItem, error)
	Update(ctx context.Context, id string, g *model.Group) error
	Delete(ctx context.Context, id string) error
	
	AddMember(ctx context.Context, groupID, userID, role string) error
	RemoveMember(ctx context.Context, groupID, userID string) error
	GetMembers(ctx context.Context, groupID string) ([]model.GroupMember, error)
	IsMember(ctx context.Context, groupID, userID string) (bool, error)
	GetMemberCount(ctx context.Context, groupID string) (int, error)
	
	AddContest(ctx context.Context, groupID, contestID string) error
	RemoveContest(ctx context.Context, groupID, contestID string) error
	GetContests(ctx context.Context, groupID string) ([]model.Contest, error)
}
```

- [ ] **Step 2: Implement group store**

```go
// internal/store/postgres/groups.go
package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type GroupStore struct {
	db *sql.DB
}

func NewGroupStore(db *sql.DB) *GroupStore {
	return &GroupStore{db: db}
}

func (s *GroupStore) Create(ctx context.Context, g *model.Group) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	err = tx.QueryRowContext(ctx,
		`INSERT INTO groups (name, description, is_public, max_members, created_by)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at, updated_at`,
		g.Name, g.Description, g.IsPublic, g.MaxMembers, g.CreatedBy,
	).Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return err
	}
	
	// Add creator as owner
	_, err = tx.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, 'owner')`,
		g.ID, g.CreatedBy)
	if err != nil {
		return err
	}
	
	return tx.Commit()
}

func (s *GroupStore) GetByID(ctx context.Context, id string) (*model.Group, error) {
	var g model.Group
	err := s.db.QueryRowContext(ctx,
		`SELECT g.id, g.name, g.description, g.is_public, g.max_members, g.created_by,
		        u.username, COUNT(gm.user_id), g.created_at, g.updated_at
		 FROM groups g
		 JOIN users u ON g.created_by = u.id
		 LEFT JOIN group_members gm ON g.id = gm.group_id
		 WHERE g.id = $1
		 GROUP BY g.id, u.username`,
		id).Scan(&g.ID, &g.Name, &g.Description, &g.IsPublic, &g.MaxMembers,
		&g.CreatedBy, &g.CreatorName, &g.MemberCount, &g.CreatedAt, &g.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *GroupStore) List(ctx context.Context, offset, limit int) ([]model.GroupListItem, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM groups WHERE is_public = true").Scan(&total)
	
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name, g.description, g.is_public, COUNT(gm.user_id), g.created_at
		 FROM groups g
		 LEFT JOIN group_members gm ON g.id = gm.group_id
		 WHERE g.is_public = true
		 GROUP BY g.id
		 ORDER BY g.created_at DESC
		 OFFSET $1 LIMIT $2`,
		offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var items []model.GroupListItem
	for rows.Next() {
		var g model.GroupListItem
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.IsPublic, &g.MemberCount, &g.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, g)
	}
	if items == nil {
		items = []model.GroupListItem{}
	}
	return items, total, nil
}

func (s *GroupStore) AddMember(ctx context.Context, groupID, userID, role string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		groupID, userID, role)
	return err
}

func (s *GroupStore) RemoveMember(ctx context.Context, groupID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`,
		groupID, userID)
	return err
}

func (s *GroupStore) GetMembers(ctx context.Context, groupID string) ([]model.GroupMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT gm.group_id, gm.user_id, u.username, gm.role, gm.joined_at
		 FROM group_members gm
		 JOIN users u ON gm.user_id = u.id
		 WHERE gm.group_id = $1
		 ORDER BY gm.joined_at`,
		groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var members []model.GroupMember
	for rows.Next() {
		var m model.GroupMember
		if err := rows.Scan(&m.GroupID, &m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if members == nil {
		members = []model.GroupMember{}
	}
	return members, nil
}

func (s *GroupStore) IsMember(ctx context.Context, groupID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)`,
		groupID, userID).Scan(&exists)
	return exists, err
}

func (s *GroupStore) GetMemberCount(ctx context.Context, groupID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM group_members WHERE group_id = $1`,
		groupID).Scan(&count)
	return count, err
}

func (s *GroupStore) AddContest(ctx context.Context, groupID, contestID string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO group_contests (group_id, contest_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		groupID, contestID)
	return err
}

func (s *GroupStore) RemoveContest(ctx context.Context, groupID, contestID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM group_contests WHERE group_id = $1 AND contest_id = $2`,
		groupID, contestID)
	return err
}

func (s *GroupStore) GetContests(ctx context.Context, groupID string) ([]model.Contest, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.title, c.type, c.start_time, c.end_time, c.visible, c.description, c.created_at
		 FROM contests c
		 JOIN group_contests gc ON c.id = gc.contest_id
		 WHERE gc.group_id = $1
		 ORDER BY c.start_time DESC`,
		groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var contests []model.Contest
	for rows.Next() {
		var c model.Contest
		if err := rows.Scan(&c.ID, &c.Title, &c.Type, &c.StartTime, &c.EndTime, &c.Visible, &c.Description, &c.CreatedAt); err != nil {
			return nil, err
		}
		contests = append(contests, c)
	}
	if contests == nil {
		contests = []model.Contest{}
	}
	return contests, nil
}

func (s *GroupStore) ListByUser(ctx context.Context, userID string) ([]model.GroupListItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name, g.description, g.is_public, COUNT(gm2.user_id), g.created_at
		 FROM groups g
		 JOIN group_members gm ON g.id = gm.group_id
		 LEFT JOIN group_members gm2 ON g.id = gm2.group_id
		 WHERE gm.user_id = $1
		 GROUP BY g.id
		 ORDER BY g.name`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var items []model.GroupListItem
	for rows.Next() {
		var g model.GroupListItem
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.IsPublic, &g.MemberCount, &g.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, g)
	}
	if items == nil {
		items = []model.GroupListItem{}
	}
	return items, nil
}

func (s *GroupStore) Update(ctx context.Context, id string, g *model.Group) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE groups SET name = $1, description = $2, is_public = $3, max_members = $4, updated_at = NOW()
		 WHERE id = $5`,
		g.Name, g.Description, g.IsPublic, g.MaxMembers, id)
	return err
}

func (s *GroupStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM groups WHERE id = $1", id)
	return err
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/groups.go
git commit -m "feat(groups): add group store"
```

---

### Task 4: Group Handler

**Files:**
- Create: `internal/api/handler/group.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Create group handler**

```go
// internal/api/handler/group.go
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tahsinarafat/aioj/internal/api/middleware"
	"github.com/tahsinarafat/aioj/internal/model"
	"github.com/tahsinarafat/aioj/internal/store/postgres"
)

type GroupHandler struct {
	store *postgres.GroupStore
}

func NewGroupHandler(s *postgres.GroupStore) *GroupHandler {
	return &GroupHandler{store: s}
}

func (h *GroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	var req model.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	if req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	
	g := &model.Group{
		Name:        req.Name,
		Description: req.Description,
		IsPublic:    req.IsPublic,
		MaxMembers:  req.MaxMembers,
		CreatedBy:   claims.UserID,
	}
	
	if err := h.store.Create(r.Context(), g); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusCreated, g)
}

func (h *GroupHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	g, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if g == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	
	respondJSON(w, http.StatusOK, g)
}

func (h *GroupHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	
	items, total, _ := h.store.List(r.Context(), offset, limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *GroupHandler) Join(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	groupID := chi.URLParam(r, "id")
	if err := h.store.AddMember(r.Context(), groupID, claims.UserID, "member"); err != nil {
		http.Error(w, "join failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

func (h *GroupHandler) Leave(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	groupID := chi.URLParam(r, "id")
	if err := h.store.RemoveMember(r.Context(), groupID, claims.UserID); err != nil {
		http.Error(w, "leave failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

func (h *GroupHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "id")
	members, err := h.store.GetMembers(r.Context(), groupID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": members})
}

func (h *GroupHandler) AddContest(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	groupID := chi.URLParam(r, "id")
	var req struct {
		ContestID string `json:"contest_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	if err := h.store.AddContest(r.Context(), groupID, req.ContestID); err != nil {
		http.Error(w, "add contest failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusOK, map[string]string{"status": "added"})
}
```

- [ ] **Step 2: Add routes**

Add to `internal/api/router.go`:

```go
r.Route("/api/groups", func(r chi.Router) {
	r.Get("/", groupH.List)
	r.Get("/{id}", groupH.GetByID)
	r.Get("/{id}/members", groupH.GetMembers)
	
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/", groupH.Create)
		r.Post("/{id}/join", groupH.Join)
		r.Post("/{id}/leave", groupH.Leave)
		r.Post("/{id}/contests", groupH.AddContest)
	})
})
```

- [ ] **Step 3: Commit**

```bash
git add internal/api/handler/group.go internal/api/router.go
git commit -m "feat(groups): add group API endpoints"
```

---

### Task 5: Frontend Group Pages

**Files:**
- Create: `web/src/pages/GroupList.tsx`
- Create: `web/src/pages/GroupDetail.tsx`
- Create: `web/src/pages/GroupCreate.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add group API calls**

Add to `web/src/lib/api.ts`:

```typescript
groups: {
    list: (offset = 0, limit = 20) =>
        request<{ data: any[]; total: number }>(`/groups?offset=${offset}&limit=${limit}`),
    get: (id: string) => request<any>(`/groups/${id}`),
    create: (data: any) => request<any>('/groups', { method: 'POST', body: JSON.stringify(data) }),
    join: (id: string) => request(`/groups/${id}/join`, { method: 'POST' }),
    leave: (id: string) => request(`/groups/${id}/leave`, { method: 'POST' }),
    members: (id: string) => request<any>(`/groups/${id}/members`),
    addContest: (id: string, contestId: string) =>
        request(`/groups/${id}/contests`, { method: 'POST', body: JSON.stringify({ contest_id: contestId }) }),
},
```

- [ ] **Step 2: Create GroupList page**

```tsx
// web/src/pages/GroupList.tsx
import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../lib/api';

export default function GroupList() {
  const [groups, setGroups] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const limit = 20;

  useEffect(() => {
    api.groups.list(offset, limit).then(d => {
      setGroups(d.data || []);
      setTotal(d.total || 0);
    }).catch(console.error);
  }, [offset]);

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Groups</h1>
        <Link to="/groups/create" className="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700">
          Create Group
        </Link>
      </div>

      <div className="grid gap-4">
        {groups.map(g => (
          <Link key={g.id} to={`/groups/${g.id}`} className="block">
            <div className="border rounded-lg p-4 hover:bg-gray-50">
              <h3 className="font-semibold">{g.name}</h3>
              <p className="text-sm text-gray-600 mt-1">{g.description}</p>
              <div className="flex gap-4 mt-2 text-xs text-gray-500">
                <span>{g.member_count} members</span>
                <span>{g.is_public ? 'Public' : 'Private'}</span>
              </div>
            </div>
          </Link>
        ))}
      </div>

      {groups.length === 0 && (
        <p className="text-center text-gray-500 py-10">No groups found.</p>
      )}
    </div>
  );
}
```

- [ ] **Step 3: Create GroupCreate page**

```tsx
// web/src/pages/GroupCreate.tsx
import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';

export default function GroupCreate() {
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [isPublic, setIsPublic] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;

    setSubmitting(true);
    try {
      const group = await api.groups.create({ name, description, is_public: isPublic });
      navigate(`/groups/${group.id}`);
    } catch (e: any) {
      alert('Failed: ' + e.message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="max-w-md mx-auto">
      <h1 className="text-2xl font-bold mb-6">Create Group</h1>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Group Name</label>
          <input
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="w-full border rounded px-3 py-2"
            required
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={3}
            className="w-full border rounded px-3 py-2"
          />
        </div>

        <div className="flex items-center">
          <input
            type="checkbox"
            id="public"
            checked={isPublic}
            onChange={(e) => setIsPublic(e.target.checked)}
            className="mr-2"
          />
          <label htmlFor="public" className="text-sm text-gray-700">Public Group</label>
        </div>

        <button
          type="submit"
          disabled={!name.trim() || submitting}
          className="w-full bg-blue-600 text-white py-2 rounded hover:bg-blue-700 disabled:opacity-50"
        >
          {submitting ? 'Creating...' : 'Create Group'}
        </button>
      </form>
    </div>
  );
}
```

- [ ] **Step 4: Create GroupDetail page**

```tsx
// web/src/pages/GroupDetail.tsx
import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { api, getAccessToken } from '../lib/api';

export default function GroupDetail() {
  const { id } = useParams<{ id: string }>();
  const [group, setGroup] = useState<any>(null);
  const [members, setMembers] = useState<any[]>([]);
  const [isMember, setIsMember] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!id) return;
    Promise.all([
      api.groups.get(id),
      api.groups.members(id),
    ]).then(([g, m]) => {
      setGroup(g);
      setMembers(m.data || []);
    }).catch(() => {}).finally(() => setLoading(false));
  }, [id]);

  const handleJoin = async () => {
    if (!id) return;
    try {
      await api.groups.join(id);
      setIsMember(true);
      setMembers([...members, { user_id: 'current' }]);
    } catch (e: any) {
      alert('Failed: ' + e.message);
    }
  };

  const handleLeave = async () => {
    if (!id) return;
    try {
      await api.groups.leave(id);
      setIsMember(false);
      setMembers(members.filter(m => m.user_id !== 'current'));
    } catch (e: any) {
      alert('Failed: ' + e.message);
    }
  };

  if (loading) return <div>Loading...</div>;
  if (!group) return <div>Group not found</div>;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-start">
        <div>
          <h1 className="text-2xl font-bold">{group.name}</h1>
          <p className="text-gray-600 mt-1">{group.description}</p>
          <div className="flex gap-4 mt-2 text-sm text-gray-500">
            <span>{group.member_count} members</span>
            <span>{group.is_public ? 'Public' : 'Private'}</span>
          </div>
        </div>
        <button
          onClick={isMember ? handleLeave : handleJoin}
          className={`px-4 py-2 rounded ${
            isMember
              ? 'bg-red-50 text-red-600 hover:bg-red-100'
              : 'bg-blue-600 text-white hover:bg-blue-700'
          }`}
        >
          {isMember ? 'Leave' : 'Join'}
        </button>
      </div>

      <div>
        <h2 className="text-lg font-semibold mb-3">Members</h2>
        <div className="border rounded-lg divide-y">
          {members.map((m, i) => (
            <div key={i} className="px-4 py-3 flex justify-between items-center">
              <span>{m.username || m.user_id}</span>
              <span className="text-xs text-gray-500 px-2 py-1 bg-gray-100 rounded">
                {m.role}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 5: Add routes to App.tsx**

```tsx
<Route path="/groups" element={<GroupList />} />
<Route path="/groups/create" element={<GroupCreate />} />
<Route path="/groups/:id" element={<GroupDetail />} />
```

- [ ] **Step 6: Add link to Navbar**

```tsx
<Link to="/groups" className="text-sm text-gray-600 hover:text-black transition-colors">Groups</Link>
```

- [ ] **Step 7: Commit**

```bash
git add web/src/pages/GroupList.tsx web/src/pages/GroupDetail.tsx web/src/pages/GroupCreate.tsx web/src/App.tsx web/src/lib/api.ts
git commit -m "feat(groups): add group frontend pages"
```

---

## Verification Checklist

After completing all tasks:

- [ ] Database migration runs successfully
- [ ] Users can create groups
- [ ] Users can join/leave groups
- [ ] Group listing shows all public groups
- [ ] Group detail shows members
- [ ] Group contests can be added
- [ ] Private groups work correctly

---

## Notes

1. **Roles**: Owner, Admin, Member with different permissions.
2. **Max members**: Optional limit to prevent abuse.
3. **Private groups**: Only visible to members.
4. **Group contests**: Exclusive contests for group members.
