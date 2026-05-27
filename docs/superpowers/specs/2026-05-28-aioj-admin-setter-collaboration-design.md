# AIOJ — Admin, Problem Setter & Codeforces Collaboration Design Spec

## 1. Goal
Address the missing platform management features:
1. **Admin Panel**: Manage user roles, approve problem setter applications, and manage VJudge bot credentials.
2. **Problem Setter Panel**: Create/edit problems, upload testcases, manage contest problems, and manage collaboration permissions (Codeforces standard).
3. **Collaboration Permissions (Codeforces Standard)**: Introduce Owner, Co-author, and Tester permissions for problems and contests to ensure privacy and collaborative testing.
4. **Auto-Migration & Database volume fix**: Fix the database migration issue causing the HTTP 500 error on registration, and automate migration execution on backend startup.

---

## 2. Postgres Volume Mapping & Migration Fixes

### 2.1 Postgres Volume Correction
In `docker-compose.yml`, restore the standard postgres Alpine data directory mapping to prevent permission and initialization errors:
```yaml
    volumes:
      - pgdata:/var/lib/postgresql/data
```

### 2.2 Dockerfile Migration Packaging
Update `Dockerfile` to copy migrations to the production container:
```dockerfile
COPY --from=builder /app/internal/store/migrations ./internal/store/migrations
```

### 2.3 Auto-Migrations on Startup
In `cmd/aioj/main.go`, invoke `golang-migrate` on startup to automatically apply any pending migrations before starting the HTTP server:
```go
import (
    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

// inside main():
slog.Info("running database migrations...")
m, err := migrate.New("file://internal/store/migrations", fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
    cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.Name))
if err != nil {
    log.Fatalf("failed to initialize migrations: %v", err)
}
if err := m.Up(); err != nil && err != migrate.ErrNoChange {
    log.Fatalf("failed to apply migrations: %v", err)
}
slog.Info("database migrations applied successfully")
```

---

## 3. Data Model & Database Schema Updates

Create a new migration file `internal/store/migrations/000002_setter_collaboration.up.sql` to add collaboration permissions, private/public problem support, and problem setter access requests:

```sql
-- 1. Add private/public visibility flag to problems table if not exists (default false)
ALTER TABLE problems ADD COLUMN IF NOT EXISTS visible BOOLEAN NOT NULL DEFAULT false;

-- 2. Problem Collaboration Permissions
CREATE TABLE IF NOT EXISTS problem_permissions (
    problem_id UUID REFERENCES problems(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    access_level VARCHAR(16) NOT NULL CHECK (access_level IN ('owner', 'co_author', 'tester')),
    PRIMARY KEY (problem_id, user_id)
);

-- 3. Contest Collaboration Permissions
CREATE TABLE IF NOT EXISTS contest_permissions (
    contest_id UUID REFERENCES contests(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    access_level VARCHAR(16) NOT NULL CHECK (access_level IN ('manager', 'tester')),
    PRIMARY KEY (contest_id, user_id)
);

-- 4. Setter Access Applications
CREATE TABLE IF NOT EXISTS setter_applications (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 5. Seed default admin user: username='admin', email='admin@aioj.net', password='admin_secret', role='admin'
INSERT INTO users (id, username, email, password_hash, role, is_bot)
VALUES ('00000000-0000-0000-0000-000000000000', 'admin', 'admin@aioj.net', '$2a$12$CMuYP1U0znkFmeE4E02nTOVTVzPeMLJMoe1fXU23PMjWy5xcDvn2i', 'admin', false)
ON CONFLICT (username) DO NOTHING;

-- Seed default user profile for the admin
INSERT INTO user_profiles (user_id, rating, problems_solved, submissions, bio)
VALUES ('00000000-0000-0000-0000-000000000000', 1500, 0, 0, 'System Administrator')
ON CONFLICT (user_id) DO NOTHING;
```

Add corresponding `down` migration in `internal/store/migrations/000002_setter_collaboration.down.sql`:
```sql
DROP TABLE IF EXISTS setter_applications CASCADE;
DROP TABLE IF EXISTS contest_permissions CASCADE;
DROP TABLE IF EXISTS problem_permissions CASCADE;
```

---

## 4. Backend Role Guards & Collaboration Logic

### 4.1 Reusable Role Middleware
Create `internal/api/middleware/role.go`:
```go
package middleware

import (
    "net/http"
    "github.com/tahsinarafat/aioj/internal/auth"
)

func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            claims := GetUserClaims(r)
            if claims == nil {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }
            allowed := false
            for _, role := range allowedRoles {
                if claims.Role == role {
                    allowed = true
                    break
                }
            }
            if !allowed {
                http.Error(w, "forbidden", http.StatusForbidden)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

### 4.2 Collaboration Check Helpers
We will implement checking functions in `internal/store/postgres/problems.go` and `internal/store/postgres/contests.go` to determine a user's permissions:

* **Problem Permissions**:
  - Check if user has read-only access (Owner, Co-author, Tester, or Admin).
  - Check if user has edit/write access (Owner, Co-author, or Admin).
* **Contest Permissions**:
  - Check if user has manager access (Creator, Manager, or Admin).
  - Check if user has tester access (Tester).

### 4.3 New Backend REST API Endpoints

#### **Admin Management**:
* `GET /api/admin/users` — List all users (Admin only).
* `PUT /api/admin/users/{id}/role` — Edit role of a user (Admin only).
* `GET /api/admin/setter-applications` — List setter applications (Admin only).
* `POST /api/admin/setter-applications/{id}/review` — Approve/reject application (Admin only).

#### **Setter Access requests**:
* `POST /api/auth/setter-apply` — Apply for Problem Setter status (Any authenticated user).
* `GET /api/auth/setter-status` — Check status of the current user's application.

#### **Problem CRUD & Sharing**:
* `GET /api/problems/{slug}` — Anyone can view if `visible = true`. Authors/Co-authors/Testers/Admins can view even if `visible = false`.
* `POST /api/problems` — Creator gets automatically added to `problem_permissions` as `'owner'`. Only `admin` and `teacher` roles allowed.
* `PUT /api/problems/{slug}` — Edit problem details. Requires `'owner'` or `'co_author'` permission or `admin` role.
* `DELETE /api/problems/{slug}` — Delete a problem. Requires `'owner'` permission or `admin` role.
* `POST /api/problems/{id}/permissions` — Share problem. Add a co-author or tester by username. Requires `'owner'` or `admin` role.
* `DELETE /api/problems/{id}/permissions/{userID}` — Remove co-author or tester. Requires `'owner'` or `admin` role.
* `GET /api/problems/{id}/permissions` — List all problem collaborators.
* `POST /api/problems/{id}/testcases` — Upload testcase file (input/output). Requires `'owner'` or `'co_author'` or `admin`.

#### **Contest Collaboration**:
* `POST /api/contests/{id}/permissions` — Add a manager/tester (Manager/Admin only).
* `DELETE /api/contests/{id}/permissions/{userID}` — Remove a contest collaborator.
* `GET /api/contests/{id}/permissions` — List contest collaborators.

---

## 5. Frontend Screens & Layout

We will build the frontend screens under `web/src/pages/` and integrate them with the router:

### 5.1 Admin Panel Dashboard (`/admin`)
- **Stats Overview**: Total users, total problems, total contests, VJudge bot account states.
- **Access Requests Center**: Table of pending problem setter requests showing applicant's username, reason, and Action buttons (Approve/Reject).
- **Users Role Grid**: Table of all registered users with role select dropdown (promoting/demoting).
- **VJudge Bot accounts**: Fields to manage CF/AtCoder credentials and review bot status.

### 5.2 Problem Setter Panel Workspace (`/setter`)
- **Work Problems List**: Lists all problems where the user has permissions, highlighting whether they are the `Owner`, `Co-author`, or `Tester`.
- **Create/Edit Problem Screen**:
  - Full editor form for slug, title, description, time limit, memory limit, sample cases, and custom checker options.
  - **Collaborators Tab**: Form to search user by username and add them as `co_author` or `tester`.
  - **Testcases Tab**: Uploader interface to upload `.in` and `.out` file pairs, displaying the list of existing testcases.
- **Access Status Alert**: For standard users, displays a card with an apply input field to write a reason and request Problem Setter permissions.

### 5.3 Contest Management Workspace (`/contests/manage`)
- **Contest Creation Form**: Name, description, start time, end time, and freeze duration.
- **Problems Tab**: Select and order problems. Supports adding private problems as long as the contest manager has access to them.
- **Collaborators Tab**: Add managers or testers to review and verify the contest.

### 5.4 Navbar Role-Based Links
- If user has `admin` role: Show `Admin` and `Setter Workspace` links.
- If user has `teacher` role: Show `Setter Workspace` link.
- If user has standard `user` role: Show `Apply as Setter` option.

---

## 6. Testing & Success Criteria

1. **Auto-Migration Verification**: Upon restarting Docker Compose, the server automatically applies all migrations. Calling `/api/auth/register` creates the user in the database with a 201 Created and return value containing JWTs.
2. **Access Control Verification**: Standard user trying to create a problem/contest gets `403 Forbidden`. Admin/Teacher successfully creates.
3. **Collaboration Verification**:
   - A private problem (visible = false) cannot be viewed by a normal user (returns 404).
   - An Owner can add a Tester to that private problem.
   - The Tester can read the statement and test cases.
   - The Tester tries to edit the statement (returns 403).
   - The Owner demotes the Tester or deletes the problem.
4. **Vite Production Build**: `npm run build` compiles without any TypeScript or bundling errors.
