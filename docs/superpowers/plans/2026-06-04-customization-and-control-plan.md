# Customization and Control Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement profile customization options (first/last name, country, city, organization, GitHub link, display email setting, hide problem tags, password change), dynamic Rankings filtering (by country/organization), Team enhancements (is_public flag, update/delete, invitations/requests), and Group updates (manager/owner contest control, invite link/code, auto/manual join policy).

**Architecture:** We will add SQL schema migrations (migration version `000049`), extend backend model definitions and postgres store queries, implement REST API endpoints in handlers/router, and integrate them with React frontend routes and layouts.

**Tech Stack:** Go (Chi router, PostgreSQL), React, TypeScript, Tailwind CSS, react-router-dom v6.

---

## Files to Create and Modify

### Database
- Create: `internal/store/migrations/000049_customization_and_control.up.sql`
- Create: `internal/store/migrations/000049_customization_and_control.down.sql`

### Backend Models & Stores
- Modify: `internal/model/user.go` (Add profile fields and profile requests/responses)
- Modify: `internal/model/team.go` (Add team details & request models)
- Modify: `internal/model/group.go` (Add group details, join policies & request models)
- Modify: `internal/store/postgres/users.go` (Update Profile retrieval/updates, Rankings filtering query)
- Modify: `internal/store/postgres/teams.go` (Update Team CRUD, member updates, join requests/invitations)
- Modify: `internal/store/postgres/groups.go` (Update Group CRUD, member roles, invite code validation)

### Backend API Handlers & Router
- Modify: `internal/api/handler/users.go` (Add GET/PUT user profile, PUT change password)
- Modify: `internal/api/handler/rankings.go` (Update Rankings query parsing for country/org)
- Modify: `internal/api/handler/team.go` (Add team update, delete, invite, request to join, respond to requests)
- Modify: `internal/api/handler/group.go` (Add group invite, respond to request, join by code, update contest permissions)
- Modify: `internal/api/router.go` (Wire all new/modified endpoints)

### Frontend
- Modify: `web/src/lib/api.ts` (API methods for user profile, team updates/invites, group invite link/requests)
- Modify: `web/src/pages/Profile.tsx` (Add Profile Settings UI tabs: Profile details, Password Change, Invites/Requests)
- Modify: `web/src/pages/Rankings.tsx` (Add Country and Organization dropdown filters)
- Modify: `web/src/pages/TeamDetail.tsx` (Add update/delete, direct invite user, list join requests with approve/reject)
- Modify: `web/src/pages/TeamCreate.tsx` (Add `is_public` toggle)
- Modify: `web/src/pages/GroupDetail.tsx` (Add manager contest/members controls, invite link/code, list join requests with approve/reject)
- Modify: `web/src/pages/GroupCreate.tsx` (Add `join_policy` option dropdown)

---

## Tasks

### Task 1: Database Migration Schema

**Files:**
- Create: `internal/store/migrations/000049_customization_and_control.up.sql`
- Create: `internal/store/migrations/000049_customization_and_control.down.sql`

- [ ] **Step 1: Write SQL UP Migration file**
Create `internal/store/migrations/000049_customization_and_control.up.sql`:
```sql
-- Profile details
ALTER TABLE user_profiles 
ADD COLUMN first_name VARCHAR(64) DEFAULT '',
ADD COLUMN last_name VARCHAR(64) DEFAULT '',
ADD COLUMN country VARCHAR(64) DEFAULT '',
ADD COLUMN city VARCHAR(64) DEFAULT '',
ADD COLUMN organization VARCHAR(128) DEFAULT '',
ADD COLUMN github_url VARCHAR(256) DEFAULT '',
ADD COLUMN show_email BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN show_tags BOOLEAN NOT NULL DEFAULT TRUE;

-- Team public visibility
ALTER TABLE teams ADD COLUMN is_public BOOLEAN NOT NULL DEFAULT TRUE;

-- Team membership status check
ALTER TABLE team_members DROP CONSTRAINT IF EXISTS team_members_role_check;
ALTER TABLE team_members ADD CONSTRAINT team_members_role_check CHECK (role IN ('owner', 'captain', 'member', 'invited', 'requested'));

-- Group invite codes and join policy
ALTER TABLE groups 
ADD COLUMN invite_code VARCHAR(8) UNIQUE,
ADD COLUMN join_policy VARCHAR(16) NOT NULL DEFAULT 'auto_approve' CHECK (join_policy IN ('auto_approve', 'manual_approve'));

-- Group membership status check
ALTER TABLE group_members DROP CONSTRAINT IF EXISTS group_members_role_check;
ALTER TABLE group_members ADD CONSTRAINT group_members_role_check CHECK (role IN ('owner', 'manager', 'member', 'invited', 'requested'));
```

- [ ] **Step 2: Write SQL DOWN Migration file**
Create `internal/store/migrations/000049_customization_and_control.down.sql`:
```sql
-- Revert profile details
ALTER TABLE user_profiles 
DROP COLUMN IF EXISTS first_name,
DROP COLUMN IF EXISTS last_name,
DROP COLUMN IF EXISTS country,
DROP COLUMN IF EXISTS city,
DROP COLUMN IF EXISTS organization,
DROP COLUMN IF EXISTS github_url,
DROP COLUMN IF EXISTS show_email,
DROP COLUMN IF EXISTS show_tags;

-- Revert teams
ALTER TABLE teams DROP COLUMN IF EXISTS is_public;
ALTER TABLE team_members DROP CONSTRAINT IF EXISTS team_members_role_check;
ALTER TABLE team_members ADD CONSTRAINT team_members_role_check CHECK (role IN ('owner', 'captain', 'member'));

-- Revert groups
ALTER TABLE groups 
DROP COLUMN IF EXISTS invite_code,
DROP COLUMN IF EXISTS join_policy;
ALTER TABLE group_members DROP CONSTRAINT IF EXISTS group_members_role_check;
ALTER TABLE group_members ADD CONSTRAINT group_members_role_check CHECK (role IN ('owner', 'admin', 'member'));
```

- [ ] **Step 3: Run migration command to apply changes**
Run: `make migrate-up`
Expected: Database migration runs successfully.

- [ ] **Step 4: Commit DB Migration files**
```bash
git add internal/store/migrations/000049_customization_and_control.*.sql
git commit -m "migration: add profile, team, and group customization columns"
```

---

### Task 2: Backend Models & API Endpoints for Profile Settings and Rankings

**Files:**
- Modify: `internal/model/user.go`
- Modify: `internal/store/postgres/users.go`
- Modify: `internal/api/handler/users.go`
- Modify: `internal/api/handler/rankings.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Update user models**
Modify `internal/model/user.go` (add fields to `UserProfile` and `PublicProfile`, and add request structs):
```go
// Add fields in UserProfile struct
type UserProfile struct {
	UserID         string `json:"user_id"`
	Rating         int    `json:"rating"`
	ProblemsSolved int    `json:"problems_solved"`
	Submissions    int    `json:"submissions"`
	Bio            string `json:"bio,omitempty"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Country        string `json:"country"`
	City           string `json:"city"`
	Organization   string `json:"organization"`
	GithubURL      string `json:"github_url"`
	ShowEmail      bool   `json:"show_email"`
	ShowTags       bool   `json:"show_tags"`
}

// Add fields to PublicProfile struct
type PublicProfile struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email,omitempty"` // populated if show_email is true
	Rating         int       `json:"rating"`
	RatingChange   int       `json:"rating_change"`
	ContestsPlayed int       `json:"contests_played"`
	ProblemsSolved int       `json:"problems_solved"`
	Bio            string    `json:"bio,omitempty"`
	AvatarURL      string    `json:"avatar_url,omitempty"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Country        string    `json:"country"`
	City           string    `json:"city"`
	Organization   string    `json:"organization"`
	GithubURL      string    `json:"github_url"`
	ShowTags       bool      `json:"show_tags"`
	CreatedAt      time.Time `json:"created_at"`
}

// Add UpdateProfileRequest & ChangePasswordRequest
type UpdateProfileRequest struct {
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Country      string `json:"country"`
	City         string `json:"city"`
	Organization string `json:"organization"`
	GithubURL    string `json:"github_url"`
	Bio          string `json:"bio"`
	AvatarURL    string `json:"avatar_url"`
	ShowEmail    bool   `json:"show_email"`
	ShowTags     bool   `json:"show_tags"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}
```

- [ ] **Step 2: Update database queries in `internal/store/postgres/users.go`**
Ensure `GetPublicProfile` parses new columns and respects `show_email`.
Ensure `ListUsersByRating` queries can filter by country/organization.
Add profile updates and password updates:
```go
func (s *UserStore) GetProfile(ctx context.Context, userID string) (*model.UserProfile, error) {
	var p model.UserProfile
	var bio, avatar sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, rating, problems_solved, submissions, bio, avatar_url,
		        first_name, last_name, country, city, organization, github_url, show_email, show_tags
		 FROM user_profiles WHERE user_id = $1`, userID).
		Scan(&p.UserID, &p.Rating, &p.ProblemsSolved, &p.Submissions, &bio, &avatar,
			&p.FirstName, &p.LastName, &p.Country, &p.City, &p.Organization, &p.GithubURL, &p.ShowEmail, &p.ShowTags)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.Bio = bio.String
	p.AvatarURL = avatar.String
	return &p, nil
}

func (s *UserStore) UpdateProfile(ctx context.Context, userID string, p *model.UpdateProfileRequest) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE user_profiles 
		 SET first_name = $1, last_name = $2, country = $3, city = $4, organization = $5, 
		     github_url = $6, bio = $7, avatar_url = $8, show_email = $9, show_tags = $10
		 WHERE user_id = $11`,
		p.FirstName, p.LastName, p.Country, p.City, p.Organization, p.GithubURL, p.Bio, p.AvatarURL, p.ShowEmail, p.ShowTags, userID)
	return err
}

func (s *UserStore) UpdatePassword(ctx context.Context, userID, newHash string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2", newHash, userID)
	return err
}

// Modify GetPublicProfile in users.go:
// Fetch the fields first_name, last_name, country, city, organization, github_url, show_email, show_tags from user_profiles
// and conditional email from users table.
```
And update `ListUsersByRating` to filter:
```go
func (s *UserStore) ListUsersByRatingFiltered(ctx context.Context, country, organization string, offset, limit int) ([]model.RankingEntry, int, error) {
	// Build dynamic query
	var countQuery = `SELECT COUNT(*) FROM users u JOIN user_profiles up ON u.id = up.user_id WHERE u.is_bot = false`
	var selectQuery = `
		SELECT u.id, u.username, up.rating, up.problems_solved,
			(SELECT rating_change FROM rating_history WHERE user_id = u.id ORDER BY created_at DESC LIMIT 1) as rc
		FROM users u
		JOIN user_profiles up ON u.id = up.user_id
		WHERE u.is_bot = false`
	
	var args []interface{}
	var placeholderIdx = 1

	if country != "" {
		countQuery += fmt.Sprintf(" AND up.country = $%d", placeholderIdx)
		selectQuery += fmt.Sprintf(" AND up.country = $%d", placeholderIdx)
		args = append(args, country)
		placeholderIdx++
	}
	if organization != "" {
		countQuery += fmt.Sprintf(" AND up.organization = $%d", placeholderIdx)
		selectQuery += fmt.Sprintf(" AND up.organization = $%d", placeholderIdx)
		args = append(args, organization)
		placeholderIdx++
	}

	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	selectQuery += fmt.Sprintf(" ORDER BY up.rating DESC OFFSET $%d LIMIT $%d", placeholderIdx, placeholderIdx+1)
	args = append(args, offset, limit)

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []model.RankingEntry
	for rows.Next() {
		var re model.RankingEntry
		var rc sql.NullInt32
		if err := rows.Scan(&re.ID, &re.Username, &re.Rating, &re.ContestsPlayed, &rc); err != nil {
			return nil, 0, err
		}
		if rc.Valid {
			re.RatingChange = int(rc.Int32)
		}
		entries = append(entries, re)
	}
	return entries, total, nil
}
```

- [ ] **Step 3: Implement handler methods in `internal/api/handler/users.go`**
Add endpoints for editing profile:
```go
func (h *UsersHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	p, err := h.store.GetProfile(r.Context(), claims.UserID)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, p)
}

func (h *UsersHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req model.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if err := h.store.UpdateProfile(r.Context(), claims.UserID, &req); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *UsersHandler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req model.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	// Verify current password first
	u, err := h.store.GetByID(r.Context(), claims.UserID)
	if err != nil || u == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	// helper/bcrypt check:
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		http.Error(w, "incorrect current password", http.StatusBadRequest)
		return
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "hash failed", http.StatusInternalServerError)
		return
	}
	if err := h.store.UpdatePassword(r.Context(), claims.UserID, string(newHash)); err != nil {
		http.Error(w, "password update failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "password updated"})
}
```

- [ ] **Step 4: Update rankings query filters in `internal/api/handler/rankings.go`**
Support country/organization filter parameters:
```go
func (h *RankingsHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	country := r.URL.Query().Get("country")
	org := r.URL.Query().Get("organization")

	items, total, err := h.userStore.ListUsersByRatingFiltered(r.Context(), country, org, offset, limit)
	if err != nil {
		http.Error(w, "query failed", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}
```

- [ ] **Step 5: Wire routes in `internal/api/router.go`**
Register user profile routes:
```go
r.Route("/api/users", func(r chi.Router) {
	r.Get("/{username}", usersH.GetByUsername)
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Get("/profile/edit", usersH.GetProfile)
		r.Put("/profile/edit", usersH.UpdateProfile)
		r.Put("/profile/password", usersH.UpdatePassword)
	})
})
```

- [ ] **Step 6: Commit Backend Profile and Rankings modifications**
```bash
git add internal/model/user.go internal/store/postgres/users.go internal/api/handler/users.go internal/api/handler/rankings.go internal/api/router.go
git commit -m "feat: profile settings and filtered rankings API endpoints"
```

---

### Task 3: Backend Database Store and API Endpoints for Teams

**Files:**
- Modify: `internal/model/team.go`
- Modify: `internal/store/postgres/teams.go`
- Modify: `internal/api/handler/team.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Modify team model structs**
Update `internal/model/team.go` to include `IsPublic` boolean:
```go
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
	IsPublic     bool      `json:"is_public"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

type RespondTeamRequest struct {
	UserID string `json:"user_id"`
	Action string `json:"action"` // "approve", "reject", "accept", "decline"
}
```

- [ ] **Step 2: Update team store queries in `internal/store/postgres/teams.go`**
Add role modification states for inviting and requesting membership, update/delete teams, list pending requests:
```go
// In Create():
// QueryRowContext: INSERT INTO teams (name, description, is_public, created_by) VALUES ($1, $2, $3, $4) RETURNING id, rating, created_at, updated_at
// ...

func (s *TeamStore) GetPendingMembers(ctx context.Context, teamID string) ([]model.TeamMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tm.team_id, tm.user_id, u.username, tm.role, tm.joined_at
		 FROM team_members tm JOIN users u ON tm.user_id = u.id
		 WHERE tm.team_id = $1 AND tm.role IN ('invited', 'requested')`, teamID)
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
	return members, nil
}

func (s *TeamStore) GetUserPendingInvites(ctx context.Context, userID string) ([]model.Team, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, t.description, t.rating, t.max_rating
		 FROM teams t JOIN team_members tm ON t.id = tm.team_id
		 WHERE tm.user_id = $1 AND tm.role = 'invited'`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var teams []model.Team
	for rows.Next() {
		var t model.Team
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Rating, &t.MaxRating); err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, nil
}

func (s *TeamStore) GetMemberRole(ctx context.Context, teamID, userID string) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx, "SELECT role FROM team_members WHERE team_id = $1 AND user_id = $2", teamID, userID).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return role, err
}

func (s *TeamStore) UpdateMemberRole(ctx context.Context, teamID, userID, role string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE team_members SET role = $1 WHERE team_id = $2 AND user_id = $3", role, teamID, userID)
	return err
}
```

- [ ] **Step 3: Implement handler methods in `internal/api/handler/team.go`**
Add PUT update, DELETE team, invite user, join requests, respond to invites:
```go
func (h *TeamHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	teamID := chi.URLParam(r, "id")
	role, err := h.store.GetMemberRole(r.Context(), teamID, claims.UserID)
	if err != nil || (role != "owner" && role != "captain" && claims.Role != "admin") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var req model.CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	t := &model.Team{Name: req.Name, Description: req.Description, IsPublic: req.IsPublic}
	if err := h.store.Update(r.Context(), teamID, t); err != nil {
		http.Error(w, "update failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *TeamHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	teamID := chi.URLParam(r, "id")
	role, err := h.store.GetMemberRole(r.Context(), teamID, claims.UserID)
	if err != nil || (role != "owner" && claims.Role != "admin") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := h.store.Delete(r.Context(), teamID); err != nil {
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *TeamHandler) Invite(w http.ResponseWriter, r *http.Request) {
	// Owner/Captain invites user by username
}

func (h *TeamHandler) RequestJoin(w http.ResponseWriter, r *http.Request) {
	// User requests to join private team
}

func (h *TeamHandler) RespondInviteRequest(w http.ResponseWriter, r *http.Request) {
	// Process accept/decline/approve/reject actions
}
```

- [ ] **Step 4: Update routes in `internal/api/router.go`**
Configure endpoints:
```go
r.Route("/api/teams", func(r chi.Router) {
	r.Get("/", teamH.List)
	r.Get("/{id}", teamH.GetByID)
	r.Get("/{id}/members", teamH.GetMembers)
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/", teamH.Create)
		r.Put("/{id}", teamH.Update)
		r.Delete("/{id}", teamH.Delete)
		r.Post("/{id}/join", teamH.Join)
		r.Post("/{id}/leave", teamH.Leave)
		r.Post("/{id}/invite", teamH.Invite)
		r.Post("/{id}/request", teamH.RequestJoin)
		r.Post("/{id}/respond", teamH.RespondInviteRequest)
		r.Get("/{id}/pending", teamH.GetPendingMembers)
	})
})
```

- [ ] **Step 5: Commit Team updates**
```bash
git add internal/model/team.go internal/store/postgres/teams.go internal/api/handler/team.go internal/api/router.go
git commit -m "feat: team update, delete, invites, and requests API endpoints"
```

---

### Task 4: Backend Database Store and API Endpoints for Groups

**Files:**
- Modify: `internal/model/group.go`
- Modify: `internal/store/postgres/groups.go`
- Modify: `internal/api/handler/group.go`
- Modify: `internal/api/router.go`

- [ ] **Step 1: Modify group model structs**
Update `internal/model/group.go` to include `InviteCode`, `JoinPolicy` and group lists:
```go
type Group struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	IsPublic    bool       `json:"is_public"`
	MaxMembers  *int       `json:"max_members,omitempty"`
	CreatedBy   string     `json:"created_by"`
	CreatorName string     `json:"creator_name,omitempty"`
	MemberCount int        `json:"member_count"`
	InviteCode  string     `json:"invite_code"`
	JoinPolicy  string     `json:"join_policy"` // "auto_approve", "manual_approve"
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
	MaxMembers  *int   `json:"max_members,omitempty"`
	JoinPolicy  string `json:"join_policy"`
}

type RespondGroupRequest struct {
	UserID string `json:"user_id"`
	Action string `json:"action"` // "approve", "reject", "accept", "decline"
}
```

- [ ] **Step 2: Update group store queries in `internal/store/postgres/groups.go`**
Add invite codes and join policy changes, pending requests lists, manager updates:
```go
// In Create():
// Generate an invite code (8 character hex or random uppercase letters/numbers)
// QueryRowContext: INSERT INTO groups (name, description, is_public, max_members, created_by, invite_code, join_policy)
// ...

func (s *GroupStore) GetPendingMembers(ctx context.Context, groupID string) ([]model.GroupMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT gm.group_id, gm.user_id, u.username, gm.role, gm.joined_at
		 FROM group_members gm JOIN users u ON gm.user_id = u.id
		 WHERE gm.group_id = $1 AND gm.role IN ('invited', 'requested')`, groupID)
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
	return members, nil
}

func (s *GroupStore) GetUserPendingInvites(ctx context.Context, userID string) ([]model.Group, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name, g.description, g.join_policy
		 FROM groups g JOIN group_members gm ON g.id = gm.group_id
		 WHERE gm.user_id = $1 AND gm.role = 'invited'`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var groups []model.Group
	for rows.Next() {
		var g model.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.JoinPolicy); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func (s *GroupStore) GetMemberRole(ctx context.Context, groupID, userID string) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx, "SELECT role FROM group_members WHERE group_id = $1 AND user_id = $2", groupID, userID).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return role, err
}

func (s *GroupStore) UpdateMemberRole(ctx context.Context, groupID, userID, role string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE group_members SET role = $1 WHERE group_id = $2 AND user_id = $3", role, groupID, userID)
	return err
}

func (s *GroupStore) GetByInviteCode(ctx context.Context, code string) (*model.Group, error) {
	var g model.Group
	err := s.db.QueryRowContext(ctx, "SELECT id, name, description, is_public, join_policy FROM groups WHERE invite_code = $1", code).
		Scan(&g.ID, &g.Name, &g.Description, &g.IsPublic, &g.JoinPolicy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &g, err
}
```

- [ ] **Step 3: Implement handler methods in `internal/api/handler/group.go`**
Implement group invitation responses, code-join routing, and restrict contest permissions:
```go
func (h *GroupHandler) Invite(w http.ResponseWriter, r *http.Request) {
	// Manager or Owner invites user by username
}

func (h *GroupHandler) RespondInviteRequest(w http.ResponseWriter, r *http.Request) {
	// Handle accept/decline/approve/reject
}

func (h *GroupHandler) JoinByCode(w http.ResponseWriter, r *http.Request) {
	// Match invite code, join with auto_approve or manual_approve policy
}

// Update AddContest/RemoveContest permissions check:
// role, _ := h.store.GetMemberRole(ctx, groupID, claims.UserID)
// if role != "owner" && role != "manager" && claims.Role != "admin" { http.Error(...) }
```

- [ ] **Step 4: Update routes in `internal/api/router.go`**
Wire new endpoints:
```go
r.Route("/api/groups", func(r chi.Router) {
	r.Get("/", groupH.List)
	r.Get("/{id}", groupH.GetByID)
	r.Get("/{id}/members", groupH.GetMembers)
	r.Get("/{id}/contests", groupH.GetContests)
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtManager))
		r.Post("/", groupH.Create)
		r.Get("/my", groupH.ListByUser)
		r.Post("/join-code", groupH.JoinByCode)
		r.Put("/{id}", groupH.Update)
		r.Delete("/{id}", groupH.Delete)
		r.Post("/{id}/join", groupH.Join)
		r.Post("/{id}/leave", groupH.Leave)
		r.Post("/{id}/contests", groupH.AddContest)
		r.Delete("/{id}/contests/{contestId}", groupH.RemoveContest)
		r.Post("/{id}/invite", groupH.Invite)
		r.Post("/{id}/respond", groupH.RespondInviteRequest)
		r.Get("/{id}/pending", groupH.GetPendingMembers)
	})
})
```

- [ ] **Step 5: Commit Group enhancements**
```bash
git add internal/model/group.go internal/store/postgres/groups.go internal/api/handler/group.go internal/api/router.go
git commit -m "feat: group invite codes, join policies, and manager contest role controls"
```

---

### Task 5: Frontend UI - Settings & Customization under `/profile`

**Files:**
- Modify: `web/src/lib/api.ts`
- Modify: `web/src/pages/Profile.tsx`

- [ ] **Step 1: Add profile endpoints in `web/src/lib/api.ts`**
Add:
```typescript
profile: {
    getEdit: () => request<any>('/users/profile/edit'),
    update: (body: any) => request<any>('/users/profile/edit', { method: 'PUT', body }),
    changePassword: (body: any) => request<any>('/users/profile/password', { method: 'PUT', body }),
    pendingInvites: () => request<any>('/users/profile/invites'), // optional endpoint if we add list, or separate team/group ones
}
```

- [ ] **Step 2: Redesign `web/src/pages/Profile.tsx`**
Create tabbed layout:
* **Tab 1: Edit Profile**: Inputs for first/last name, country, city, organization, GitHub username, avatar URL, bio, show_email checkbox, show_tags checkbox.
* **Tab 2: Password**: Inputs for current and new password with submit button.
* **Tab 3: Invites & Requests**: Lists pending team and group invitations with "Accept" and "Decline" buttons.

- [ ] **Step 3: Commit Frontend Profile settings**
```bash
git add web/src/lib/api.ts web/src/pages/Profile.tsx
git commit -m "feat: client profile settings tab layout with details and password update"
```

---

### Task 4: Frontend UI - Rankings Filtering by country/organization

**Files:**
- Modify: `web/src/pages/Rankings.tsx`

- [ ] **Step 1: Update rankings page filter controls**
Add country dropdown list and organization query input to `Rankings.tsx`:
```typescript
const [selectedCountry, setSelectedCountry] = useState('');
const [selectedOrg, setSelectedOrg] = useState('');
```
Add UI elements above the table, hook up filters to api call, updating query logic to trigger fetch when filters change.

- [ ] **Step 2: Commit Rankings page updates**
```bash
git add web/src/pages/Rankings.tsx
git commit -m "feat: add country and organization filtering widgets on rankings board"
```

---

### Task 7: Frontend UI - Team settings, private invite & request system

**Files:**
- Modify: `web/src/pages/TeamDetail.tsx`
- Modify: `web/src/pages/TeamCreate.tsx`

- [ ] **Step 1: Add is_public to Team Create**
Update `TeamCreate.tsx` with a toggle checkbox for "Private Team".

- [ ] **Step 2: Add management controls to Team Detail**
* Update team join button logic for private teams: Show "Request to Join" button which triggers `POST /api/teams/{id}/request` instead of joining directly.
* Show an edit/delete modal/form to the team captain/owner.
* Show invite user field to invite user directly by username.
* Add "Pending Join Requests" table to team captain showing users requesting with "Approve" / "Reject" buttons.

- [ ] **Step 3: Commit Teams UI updates**
```bash
git add web/src/pages/TeamDetail.tsx web/src/pages/TeamCreate.tsx
git commit -m "feat: interactive private team invites, settings management, and join requests UI"
```

---

### Task 8: Frontend UI - Group settings, private invite code/link & request system

**Files:**
- Modify: `web/src/pages/GroupDetail.tsx`
- Modify: `web/src/pages/GroupCreate.tsx`

- [ ] **Step 1: Add Join Policy selection to Group Create**
Modify `GroupCreate.tsx` to include "Join Policy" dropdown selecting "Auto-Approve" or "Manual Approve".

- [ ] **Step 2: Add managers, invite codes & pending requests to Group Detail**
* Display group invite code and copy link button to group managers and owners.
* Display direct search/invite users panel for manager/owner.
* Add "Join Requests" tab showing pending join requests with "Approve" / "Reject" actions.
* Update join button for manual-approve groups: Click "Request to Join" to issue request.
* Update contest management validation on frontend: Allow adding contests if user is owner/manager.

- [ ] **Step 3: Commit Groups UI updates**
```bash
git add web/src/pages/GroupDetail.tsx web/src/pages/GroupCreate.tsx
git commit -m "feat: group invite link sharing, join policies, and manager approval UI"
```
