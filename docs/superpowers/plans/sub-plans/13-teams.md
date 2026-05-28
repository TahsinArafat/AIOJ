# Sub-Plan 13: Teams

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to form teams and participate in team contests with team ratings.

**Architecture:** Add `teams` and `team_members` tables, team contest participation, team rating system.

**Tech Stack:** Go, PostgreSQL, React, TypeScript

---

## File Structure

### Backend Files to Create
- `internal/model/team.go` - Team models
- `internal/store/postgres/teams.go` - Team store
- `internal/api/handler/team.go` - Team handler

### Backend Files to Modify
- `internal/store/interfaces.go` - Add TeamStore interface
- `internal/api/router.go` - Add team routes

### Frontend Files to Create
- `web/src/pages/TeamList.tsx` - Team listing
- `web/src/pages/TeamDetail.tsx` - Team detail page
- `web/src/pages/TeamCreate.tsx` - Create team form

### Frontend Files to Modify
- `web/src/App.tsx` - Add team routes
- `web/src/lib/api.ts` - Add team API calls

---

## Tasks

### Task 1: Database Migration

**Files:**
- Create: `internal/store/migrations/000011_teams.up.sql`
- Create: `internal/store/migrations/000011_teams.down.sql`

- [ ] **Step 1: Create migration up file**

```sql
-- internal/store/migrations/000011_teams.up.sql

CREATE TABLE teams (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    avatar_url VARCHAR(512),
    rating INTEGER NOT NULL DEFAULT 1500,
    max_rating INTEGER NOT NULL DEFAULT 1500,
    contest_count INTEGER NOT NULL DEFAULT 0,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE team_members (
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(16) NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'captain', 'member')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (team_id, user_id)
);

CREATE TABLE team_contests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id),
    contest_id UUID NOT NULL REFERENCES contests(id),
    rank INTEGER,
    score INTEGER NOT NULL DEFAULT 0,
    rating_change INTEGER NOT NULL DEFAULT 0,
    participated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE team_rating_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id UUID NOT NULL REFERENCES teams(id),
    contest_id UUID NOT NULL REFERENCES contests(id),
    old_rating INTEGER NOT NULL,
    new_rating INTEGER NOT NULL,
    rating_change INTEGER NOT NULL,
    rank INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_team_members_user ON team_members(user_id);
CREATE INDEX idx_team_contests_team ON team_contests(team_id);
CREATE INDEX idx_team_rating_history_team ON team_rating_history(team_id);
```

- [ ] **Step 2: Create migration down file**

```sql
-- internal/store/migrations/000011_teams.down.sql

DROP TABLE IF EXISTS team_rating_history;
DROP TABLE IF EXISTS team_contests;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
```

- [ ] **Step 3: Run migration**

Run: `make migrate-up`

- [ ] **Step 4: Commit**

```bash
git add internal/store/migrations/000011_teams.*
git commit -m "feat(teams): add teams database migration"
```

---

### Task 2: Team Models

**Files:**
- Create: `internal/model/team.go`

- [ ] **Step 1: Create team models**

```go
// internal/model/team.go
package model

import "time"

type Team struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	AvatarURL    string    `json:"avatar_url,omitempty"`
	Rating       int       `json:"rating"`
	MaxRating    int       `json:"max_rating"`
	ContestCount int       `json:"contest_count"`
	MemberCount  int       `json:"member_count"`
	CreatedBy    string    `json:"created_by"`
	CreatorName  string    `json:"creator_name,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TeamMember struct {
	TeamID    string    `json:"team_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username,omitempty"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

type TeamContest struct {
	ID           string    `json:"id"`
	TeamID       string    `json:"team_id"`
	ContestID    string    `json:"contest_id"`
	ContestTitle string    `json:"contest_title,omitempty"`
	Rank         *int      `json:"rank,omitempty"`
	Score        int       `json:"score"`
	RatingChange int       `json:"rating_change"`
	ParticipatedAt time.Time `json:"participated_at"`
}

type CreateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type TeamListItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Rating      int       `json:"rating"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/team.go
git commit -m "feat(teams): add team models"
```

---

### Task 3: Team Store

**Files:**
- Create: `internal/store/postgres/teams.go`
- Modify: `internal/store/interfaces.go`

- [ ] **Step 1: Add TeamStore interface**

```go
type TeamStore interface {
	Create(ctx context.Context, t *model.Team) error
	GetByID(ctx context.Context, id string) (*model.Team, error)
	List(ctx context.Context, offset, limit int) ([]model.TeamListItem, int, error)
	Update(ctx context.Context, id string, t *model.Team) error
	Delete(ctx context.Context, id string) error
	
	AddMember(ctx context.Context, teamID, userID, role string) error
	RemoveMember(ctx context.Context, teamID, userID string) error
	GetMembers(ctx context.Context, teamID string) ([]model.TeamMember, error)
	IsMember(ctx context.Context, teamID, userID string) (bool, error)
	GetUserTeams(ctx context.Context, userID string) ([]model.TeamListItem, error)
	
	AddContest(ctx context.Context, tc *model.TeamContest) error
	GetContests(ctx context.Context, teamID string) ([]model.TeamContest, error)
	UpdateRating(ctx context.Context, teamID string, newRating int) error
}
```

- [ ] **Step 2: Implement team store**

```go
// internal/store/postgres/teams.go
package postgres

import (
	"context"
	"database/sql"

	"github.com/tahsinarafat/aioj/internal/model"
)

type TeamStore struct {
	db *sql.DB
}

func NewTeamStore(db *sql.DB) *TeamStore {
	return &TeamStore{db: db}
}

func (s *TeamStore) Create(ctx context.Context, t *model.Team) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	
	err = tx.QueryRowContext(ctx,
		`INSERT INTO teams (name, description, created_by) VALUES ($1, $2, $3) RETURNING id, rating, created_at, updated_at`,
		t.Name, t.Description, t.CreatedBy,
	).Scan(&t.ID, &t.Rating, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return err
	}
	
	_, err = tx.ExecContext(ctx,
		"INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, 'owner')",
		t.ID, t.CreatedBy)
	if err != nil {
		return err
	}
	
	return tx.Commit()
}

func (s *TeamStore) GetByID(ctx context.Context, id string) (*model.Team, error) {
	var t model.Team
	err := s.db.QueryRowContext(ctx,
		`SELECT t.id, t.name, t.description, t.avatar_url, t.rating, t.max_rating, 
		        t.contest_count, COUNT(tm.user_id), t.created_by, u.username, t.created_at, t.updated_at
		 FROM teams t
		 JOIN users u ON t.created_by = u.id
		 LEFT JOIN team_members tm ON t.id = tm.team_id
		 WHERE t.id = $1
		 GROUP BY t.id, u.username`,
		id).Scan(&t.ID, &t.Name, &t.Description, &t.AvatarURL, &t.Rating, &t.MaxRating,
		&t.ContestCount, &t.MemberCount, &t.CreatedBy, &t.CreatorName, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *TeamStore) List(ctx context.Context, offset, limit int) ([]model.TeamListItem, int, error) {
	var total int
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM teams").Scan(&total)
	
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, t.rating, COUNT(tm.user_id), t.created_at
		 FROM teams t LEFT JOIN team_members tm ON t.id = tm.team_id
		 GROUP BY t.id ORDER BY t.rating DESC OFFSET $1 LIMIT $2`,
		offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	
	var items []model.TeamListItem
	for rows.Next() {
		var t model.TeamListItem
		if err := rows.Scan(&t.ID, &t.Name, &t.Rating, &t.MemberCount, &t.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, t)
	}
	if items == nil {
		items = []model.TeamListItem{}
	}
	return items, total, nil
}

func (s *TeamStore) AddMember(ctx context.Context, teamID, userID, role string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO team_members (team_id, user_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		teamID, userID, role)
	return err
}

func (s *TeamStore) RemoveMember(ctx context.Context, teamID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM team_members WHERE team_id = $1 AND user_id = $2",
		teamID, userID)
	return err
}

func (s *TeamStore) GetMembers(ctx context.Context, teamID string) ([]model.TeamMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tm.team_id, tm.user_id, u.username, tm.role, tm.joined_at
		 FROM team_members tm JOIN users u ON tm.user_id = u.id
		 WHERE tm.team_id = $1 ORDER BY tm.joined_at`,
		teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var members []model.TeamMember
	for rows.Next() {
		var m model.TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if members == nil {
		members = []model.TeamMember{}
	}
	return members, nil
}

func (s *TeamStore) IsMember(ctx context.Context, teamID, userID string) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM team_members WHERE team_id = $1 AND user_id = $2)",
		teamID, userID).Scan(&exists)
	return exists, err
}

func (s *TeamStore) GetUserTeams(ctx context.Context, userID string) ([]model.TeamListItem, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, t.rating, COUNT(tm2.user_id), t.created_at
		 FROM teams t
		 JOIN team_members tm ON t.id = tm.team_id
		 LEFT JOIN team_members tm2 ON t.id = tm2.team_id
		 WHERE tm.user_id = $1 GROUP BY t.id ORDER BY t.name`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var items []model.TeamListItem
	for rows.Next() {
		var t model.TeamListItem
		if err := rows.Scan(&t.ID, &t.Name, &t.Rating, &t.MemberCount, &t.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	if items == nil {
		items = []model.TeamListItem{}
	}
	return items, nil
}

func (s *TeamStore) UpdateRating(ctx context.Context, teamID string, newRating int) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE teams SET rating = $1, max_rating = GREATEST(max_rating, $1), updated_at = NOW() WHERE id = $2",
		newRating, teamID)
	return err
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/store/interfaces.go internal/store/postgres/teams.go
git commit -m "feat(teams): add team store"
```

---

### Task 4: Team Handler and Frontend

**Files:**
- Create: `internal/api/handler/team.go`
- Modify: `internal/api/router.go`
- Create: `web/src/pages/TeamList.tsx`
- Create: `web/src/pages/TeamDetail.tsx`
- Modify: `web/src/App.tsx`
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Create team handler**

```go
// internal/api/handler/team.go
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

type TeamHandler struct {
	store *postgres.TeamStore
}

func NewTeamHandler(s *postgres.TeamStore) *TeamHandler {
	return &TeamHandler{store: s}
}

func (h *TeamHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	var req model.CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	
	t := &model.Team{
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   claims.UserID,
	}
	
	if err := h.store.Create(r.Context(), t); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}
	
	respondJSON(w, http.StatusCreated, t)
}

func (h *TeamHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	
	items, total, _ := h.store.List(r.Context(), offset, limit)
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

func (h *TeamHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	t, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if t == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	respondJSON(w, http.StatusOK, t)
}

func (h *TeamHandler) Join(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	teamID := chi.URLParam(r, "id")
	if err := h.store.AddMember(r.Context(), teamID, claims.UserID, "member"); err != nil {
		http.Error(w, "join failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

func (h *TeamHandler) Leave(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	
	teamID := chi.URLParam(r, "id")
	if err := h.store.RemoveMember(r.Context(), teamID, claims.UserID); err != nil {
		http.Error(w, "leave failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

func (h *TeamHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	members, err := h.store.GetMembers(r.Context(), teamID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"data": members})
}
```

- [ ] **Step 2: Add routes**

```go
r.Route("/api/teams", func(r chi.Router) {
	r.Get("/", teamH.List)
	r.Get("/{id}", teamH.GetByID)
	r.Get("/{id}/members", teamH.GetMembers)
	
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/", teamH.Create)
		r.Post("/{id}/join", teamH.Join)
		r.Post("/{id}/leave", teamH.Leave)
	})
})
```

- [ ] **Step 3: Add team API calls**

```typescript
teams: {
    list: (offset = 0, limit = 20) =>
        request<{ data: any[]; total: number }>(`/teams?offset=${offset}&limit=${limit}`),
    get: (id: string) => request<any>(`/teams/${id}`),
    create: (data: any) => request<any>('/teams', { method: 'POST', body: JSON.stringify(data) }),
    join: (id: string) => request(`/teams/${id}/join`, { method: 'POST' }),
    leave: (id: string) => request(`/teams/${id}/leave`, { method: 'POST' }),
    members: (id: string) => request<any>(`/teams/${id}/members`),
},
```

- [ ] **Step 4: Create TeamList page**

```tsx
// web/src/pages/TeamList.tsx
import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../lib/api';
import RatingBadge from '../components/RatingBadge';

export default function TeamList() {
  const [teams, setTeams] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const limit = 20;

  useEffect(() => {
    api.teams.list(offset, limit).then(d => {
      setTeams(d.data || []);
      setTotal(d.total || 0);
    }).catch(console.error);
  }, [offset]);

  return (
    <div>
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Teams</h1>
        <Link to="/teams/create" className="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700">
          Create Team
        </Link>
      </div>

      <div className="border rounded-lg divide-y">
        {teams.map(t => (
          <Link key={t.id} to={`/teams/${t.id}`} className="flex items-center justify-between px-4 py-3 hover:bg-gray-50">
            <div>
              <h3 className="font-medium">{t.name}</h3>
              <p className="text-sm text-gray-500">{t.member_count} members</p>
            </div>
            <RatingBadge rating={t.rating} size="sm" />
          </Link>
        ))}
      </div>
    </div>
  );
}
```

- [ ] **Step 5: Create TeamDetail page**

```tsx
// web/src/pages/TeamDetail.tsx
import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import { api, getAccessToken } from '../lib/api';
import RatingBadge from '../components/RatingBadge';

export default function TeamDetail() {
  const { id } = useParams<{ id: string }>();
  const [team, setTeam] = useState<any>(null);
  const [members, setMembers] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!id) return;
    Promise.all([
      api.teams.get(id),
      api.teams.members(id),
    ]).then(([t, m]) => {
      setTeam(t);
      setMembers(m.data || []);
    }).catch(() => {}).finally(() => setLoading(false));
  }, [id]);

  const handleJoin = async () => {
    if (!id) return;
    await api.teams.join(id);
    setMembers([...members, { user_id: 'current' }]);
  };

  if (loading) return <div>Loading...</div>;
  if (!team) return <div>Team not found</div>;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-start">
        <div>
          <h1 className="text-2xl font-bold">{team.name}</h1>
          <p className="text-gray-600 mt-1">{team.description}</p>
        </div>
        <RatingBadge rating={team.rating} showTitle />
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div className="bg-gray-50 p-4 rounded-lg text-center">
          <p className="text-2xl font-bold">{team.rating}</p>
          <p className="text-sm text-gray-500">Rating</p>
        </div>
        <div className="bg-gray-50 p-4 rounded-lg text-center">
          <p className="text-2xl font-bold">{team.max_rating}</p>
          <p className="text-sm text-gray-500">Max Rating</p>
        </div>
        <div className="bg-gray-50 p-4 rounded-lg text-center">
          <p className="text-2xl font-bold">{team.contest_count}</p>
          <p className="text-sm text-gray-500">Contests</p>
        </div>
      </div>

      <div>
        <h2 className="text-lg font-semibold mb-3">Members</h2>
        <div className="border rounded-lg divide-y">
          {members.map(m => (
            <div key={m.user_id} className="px-4 py-3 flex justify-between">
              <span>{m.username}</span>
              <span className="text-sm text-gray-500">{m.role}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 6: Add routes**

```tsx
<Route path="/teams" element={<TeamList />} />
<Route path="/teams/create" element={<TeamCreate />} />
<Route path="/teams/:id" element={<TeamDetail />} />
```

- [ ] **Step 7: Commit**

```bash
git add internal/api/handler/team.go internal/api/router.go web/src/pages/Team*.tsx web/src/App.tsx web/src/lib/api.ts
git commit -m "feat(teams): add team functionality"
```

---

## Verification Checklist

- [ ] Teams can be created
- [ ] Users can join/leave teams
- [ ] Team rating displays correctly
- [ ] Team members list shows
- [ ] Team list sorted by rating

---

## Notes

1. **Team size**: Default max 3 members (configurable)
2. **Team rating**: Calculated from contest participation
3. **Roles**: Owner, Captain, Member
4. **Team contests**: Special contests for teams only
