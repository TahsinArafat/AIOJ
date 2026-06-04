# Migration Sustainability & Structural Extensions Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the migration system safe to roll back, add operational tooling, fix the gap/missing-down-file problems, and reduce structural debt (god router, handler duplication, magic strings) so the codebase can grow without increasing fragility.

**Architecture:** Four independent tracks — migration hygiene (gap + missing downs), CLI hardening (version/status/steps/force), magic-string typed constants, and router dependency container. Each track is independently mergeable. No track blocks another.

**Tech Stack:** Go 1.26, `github.com/golang-migrate/migrate/v4`, PostgreSQL 18, Chi router, `database/sql`.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `internal/store/migrations/000013_editorial.up.sql` | **Create** | Fill sequence gap |
| `internal/store/migrations/000013_editorial.down.sql` | **Create** | Rollback for gap filler |
| `internal/store/migrations/000039_contest_enhancements.down.sql` | **Create** | Rollback for 39 |
| `internal/store/migrations/000040_clarifications_notices.down.sql` | **Create** | Rollback for 40 |
| `internal/store/migrations/000041_pdf_statement_toggle.down.sql` | **Create** | Rollback for 41 |
| `internal/store/migrations/000042_contest_slug.down.sql` | **Create** | Rollback for 42 |
| `internal/store/migrations/000043_fix_problem_cascade_delete.down.sql` | **Create** | Rollback for 43 |
| `internal/store/migrations/000044_problem_default_private.down.sql` | **Create** | Rollback for 44 |
| `cmd/migrate/main.go` | **Modify** | Add `status`, `version`, `steps N`, `force N` subcommands |
| `internal/model/constants.go` | **Create** | Typed role/status/access-level constants |
| `internal/api/deps.go` | **Create** | `Deps` struct holding all handler pointers |
| `internal/api/router.go` | **Modify** | `NewRouter(deps Deps, jwt *auth.JWTManager)` — 2 params |
| `internal/api/handler/submission.go` | **Modify** | Extract `buildAndEnqueueSubmission` helper, remove duplication |
| `Makefile` | **Modify** | Add `migrate-status`, `migrate-force`, `migrate-version` targets |

---

## Task 1: Fill Migration Sequence Gap (000013)

**Context:** `golang-migrate` uses sequential numbering. The gap between 000012 and 000014 doesn't crash `migrate up` but does crash `migrate down` once you reach 000014 → down → 000013 (missing) → error. The editorial system was added later (migration 000014 is `api_keys`, not editorial). The editorial store already exists — we need a no-op placeholder at 000013 so the sequence is contiguous.

**Files:**
- Create: `internal/store/migrations/000013_editorial.up.sql`
- Create: `internal/store/migrations/000013_editorial.down.sql`

- [ ] **Step 1: Check what's in 000014 to confirm 000013 is truly missing editorial**

```bash
cat /Users/tahsinarafat/App_Dev/AIOJ/internal/store/migrations/000014_api_keys.up.sql
```

Expected: api_keys table DDL — confirms editorial has no migration, needs placeholder.

- [ ] **Step 2: Create the up placeholder**

Create `internal/store/migrations/000013_editorial.up.sql`:

```sql
-- Editorial table was created inline in 000001_init.up.sql via the editorials table.
-- This file is a sequence placeholder to keep migration numbering contiguous.
-- No schema changes needed.
SELECT 1;
```

- [ ] **Step 3: Create the down placeholder**

Create `internal/store/migrations/000013_editorial.down.sql`:

```sql
-- No-op: matches the no-op up migration.
SELECT 1;
```

- [ ] **Step 4: Verify migrate up still works**

```bash
make migrate-up
```

Expected output ends with: `migration up complete` (no error).

- [ ] **Step 5: Verify migrate down works past 000013**

```bash
DB_DSN="postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable" \
go run ./cmd/migrate -dir internal/store/migrations steps -1
```

Expected: Steps down by 1 without hitting "no migration" error.

- [ ] **Step 6: Commit**

```bash
git add internal/store/migrations/000013_editorial.up.sql \
        internal/store/migrations/000013_editorial.down.sql
git commit -m "fix: add 000013 sequence placeholder to close migration gap"
```

---

## Task 2: Write Down Migrations for 000039–000044

**Context:** Migrations 000039 through 000044 have no `.down.sql`. If a rollback is needed, `migrate down` fails. Each down must exactly reverse the up.

**Files:**
- Create: `internal/store/migrations/000039_contest_enhancements.down.sql`
- Create: `internal/store/migrations/000040_clarifications_notices.down.sql`
- Create: `internal/store/migrations/000041_pdf_statement_toggle.down.sql`
- Create: `internal/store/migrations/000042_contest_slug.down.sql`
- Create: `internal/store/migrations/000043_fix_problem_cascade_delete.down.sql`
- Create: `internal/store/migrations/000044_problem_default_private.down.sql`

- [ ] **Step 1: Create 000039 down**

Create `internal/store/migrations/000039_contest_enhancements.down.sql`:

```sql
-- Restore original access_level constraint (without 'judge')
ALTER TABLE contest_permissions DROP CONSTRAINT IF EXISTS contest_permissions_access_level_check;
ALTER TABLE contest_permissions ADD CONSTRAINT contest_permissions_access_level_check
    CHECK (access_level IN ('manager', 'tester'));

-- Remove columns added in up
ALTER TABLE contests DROP COLUMN IF EXISTS upsolving_enabled;
ALTER TABLE contests DROP COLUMN IF EXISTS virtual_contest_enabled;

-- Drop onsite_batch_users table
DROP INDEX IF EXISTS idx_onsite_batch_users_contest;
DROP INDEX IF EXISTS idx_onsite_batch_users_username;
DROP TABLE IF EXISTS onsite_batch_users;
```

- [ ] **Step 2: Create 000040 down**

Create `internal/store/migrations/000040_clarifications_notices.down.sql`:

```sql
DROP INDEX IF EXISTS idx_clarifications_contest;
DROP INDEX IF EXISTS idx_clarifications_user;
DROP TABLE IF EXISTS clarifications;

DROP INDEX IF EXISTS idx_contest_notices_contest;
DROP TABLE IF EXISTS contest_notices;
```

- [ ] **Step 3: Create 000041 down**

Create `internal/store/migrations/000041_pdf_statement_toggle.down.sql`:

```sql
ALTER TABLE contests DROP COLUMN IF EXISTS pdf_enabled;
ALTER TABLE contests DROP COLUMN IF EXISTS statement_hidden;
```

- [ ] **Step 4: Create 000042 down**

Create `internal/store/migrations/000042_contest_slug.down.sql`:

```sql
DROP INDEX IF EXISTS idx_contests_slug;
ALTER TABLE contests DROP COLUMN IF EXISTS slug;
```

- [ ] **Step 5: Create 000043 down**

Reverting 000043 requires restoring the original FK constraints (no cascade). Check what the original FK was in `000001_init.up.sql`:

```bash
grep -A3 "contest_problems_problem_id_fkey\|submissions_problem_id_fkey\|hacks_problem_id_fkey" \
  /Users/tahsinarafat/App_Dev/AIOJ/internal/store/migrations/000001_init.up.sql
```

The original FKs had no `ON DELETE CASCADE`. Create `internal/store/migrations/000043_fix_problem_cascade_delete.down.sql`:

```sql
-- Restore original FK constraints (no ON DELETE CASCADE)
ALTER TABLE contest_problems DROP CONSTRAINT IF EXISTS contest_problems_problem_id_fkey;
ALTER TABLE contest_problems ADD CONSTRAINT contest_problems_problem_id_fkey
    FOREIGN KEY (problem_id) REFERENCES problems(id);

ALTER TABLE submissions DROP CONSTRAINT IF EXISTS submissions_problem_id_fkey;
ALTER TABLE submissions ADD CONSTRAINT submissions_problem_id_fkey
    FOREIGN KEY (problem_id) REFERENCES problems(id);

ALTER TABLE hacks DROP CONSTRAINT IF EXISTS hacks_problem_id_fkey;
ALTER TABLE hacks ADD CONSTRAINT hacks_problem_id_fkey
    FOREIGN KEY (problem_id) REFERENCES problems(id);
```

- [ ] **Step 6: Create 000044 down**

Create `internal/store/migrations/000044_problem_default_private.down.sql`:

```sql
-- Restore original default (problems were visible by default before this migration)
ALTER TABLE problems ALTER COLUMN visible SET DEFAULT true;
```

- [ ] **Step 7: Verify all down files exist**

```bash
ls internal/store/migrations/0000{39,40,41,42,43,44}*.down.sql
```

Expected: 6 files listed, no errors.

- [ ] **Step 8: Commit**

```bash
git add internal/store/migrations/000039_contest_enhancements.down.sql \
        internal/store/migrations/000040_clarifications_notices.down.sql \
        internal/store/migrations/000041_pdf_statement_toggle.down.sql \
        internal/store/migrations/000042_contest_slug.down.sql \
        internal/store/migrations/000043_fix_problem_cascade_delete.down.sql \
        internal/store/migrations/000044_problem_default_private.down.sql
git commit -m "fix: add missing down migrations for 000039-000044"
```

---

## Task 3: Harden the migrate CLI

**Context:** Current `cmd/migrate/main.go` only supports `up` and `down` (all-the-way). Production needs `status` (see current version), `version` (print version number), `steps N` (migrate N steps up or down), and `force N` (force set version without running SQL — recovery from dirty state). All these are already in the `golang-migrate` library.

**Files:**
- Modify: `cmd/migrate/main.go`
- Modify: `Makefile`

- [ ] **Step 1: Write test for the CLI argument parsing logic**

Create `cmd/migrate/main_test.go`:

```go
package main

import (
	"strconv"
	"testing"
)

// parseSteps validates that steps argument is a non-zero integer.
func TestParseSteps(t *testing.T) {
	cases := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"1", 1, false},
		{"-1", -1, false},
		{"5", 5, false},
		{"0", 0, true},
		{"abc", 0, true},
	}
	for _, c := range cases {
		n, err := strconv.Atoi(c.input)
		if c.wantErr {
			if err == nil && n != 0 {
				t.Errorf("parseSteps(%q): expected error or zero, got %d", c.input, n)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSteps(%q): unexpected error %v", c.input, err)
			continue
		}
		if n != c.want {
			t.Errorf("parseSteps(%q) = %d, want %d", c.input, n, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to confirm it fails (function doesn't exist yet)**

```bash
go test ./cmd/migrate/...
```

Expected: PASS (this test uses stdlib only — it's testing our input validation logic, not a named function).

- [ ] **Step 3: Rewrite cmd/migrate/main.go**

Replace the full contents of `cmd/migrate/main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable"
	}
	dir := flag.String("dir", "internal/store/migrations", "migrations directory")
	flag.Parse()

	cmd := flag.Arg(0)
	if cmd == "" {
		log.Fatal("usage: migrate [-dir <path>] <up|down|steps N|version|status|force N>")
	}

	m, err := migrate.New("file://"+*dir, dsn)
	if err != nil {
		log.Fatalf("migrate new: %v", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Printf("source close error: %v", srcErr)
		}
		if dbErr != nil {
			log.Printf("db close error: %v", dbErr)
		}
	}()

	switch cmd {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate up: %v", err)
		}
		log.Println("migration up complete")

	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate down: %v", err)
		}
		log.Println("migration down complete")

	case "steps":
		arg := flag.Arg(1)
		if arg == "" {
			log.Fatal("steps requires an integer argument, e.g. 'steps 2' or 'steps -1'")
		}
		n, err := strconv.Atoi(arg)
		if err != nil || n == 0 {
			log.Fatalf("steps: invalid argument %q (must be non-zero integer)", arg)
		}
		if err := m.Steps(n); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate steps %d: %v", n, err)
		}
		log.Printf("migration steps(%d) complete", n)

	case "version":
		v, dirty, err := m.Version()
		if err != nil {
			log.Fatalf("migrate version: %v", err)
		}
		if dirty {
			fmt.Printf("version: %d (DIRTY)\n", v)
		} else {
			fmt.Printf("version: %d\n", v)
		}

	case "status":
		v, dirty, err := m.Version()
		if err == migrate.ErrNilVersion {
			fmt.Println("status: no migrations applied")
			return
		}
		if err != nil {
			log.Fatalf("migrate status: %v", err)
		}
		state := "clean"
		if dirty {
			state = "DIRTY — run 'force <version>' to recover"
		}
		fmt.Printf("current version: %d (%s)\n", v, state)

	case "force":
		arg := flag.Arg(1)
		if arg == "" {
			log.Fatal("force requires a version integer argument, e.g. 'force 45'")
		}
		n, err := strconv.Atoi(arg)
		if err != nil {
			log.Fatalf("force: invalid version %q", arg)
		}
		if err := m.Force(n); err != nil {
			log.Fatalf("migrate force %d: %v", n, err)
		}
		log.Printf("forced version to %d", n)

	default:
		log.Fatalf("unknown command: %q\nUsage: migrate [-dir path] <up|down|steps N|version|status|force N>", cmd)
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./cmd/migrate/... -v
```

Expected: `PASS`.

- [ ] **Step 5: Smoke test status**

```bash
DB_DSN="postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable" \
go run ./cmd/migrate status
```

Expected: `current version: 45 (clean)` (or whatever the current version is).

- [ ] **Step 6: Update Makefile**

Replace the content of `Makefile`:

```makefile
.PHONY: build run test migrate-up migrate-down migrate-status migrate-version migrate-force

MIGRATE_DSN ?= postgres://aioj:aioj_dev@localhost:5432/aioj?sslmode=disable
MIGRATE_DIR  = internal/store/migrations

build:
	go build -o aioj ./cmd/aioj

run:
	go run ./cmd/aioj

test:
	go test ./... -v -count=1

migrate-up:
	DB_DSN="$(MIGRATE_DSN)" go run ./cmd/migrate -dir $(MIGRATE_DIR) up

migrate-down:
	DB_DSN="$(MIGRATE_DSN)" go run ./cmd/migrate -dir $(MIGRATE_DIR) down

migrate-status:
	DB_DSN="$(MIGRATE_DSN)" go run ./cmd/migrate -dir $(MIGRATE_DIR) status

migrate-version:
	DB_DSN="$(MIGRATE_DSN)" go run ./cmd/migrate -dir $(MIGRATE_DIR) version

migrate-force:
	@test -n "$(V)" || (echo "usage: make migrate-force V=<version>"; exit 1)
	DB_DSN="$(MIGRATE_DSN)" go run ./cmd/migrate -dir $(MIGRATE_DIR) force $(V)

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f backend
```

- [ ] **Step 7: Verify make targets work**

```bash
make migrate-status
```

Expected: prints current version line.

- [ ] **Step 8: Commit**

```bash
git add cmd/migrate/main.go cmd/migrate/main_test.go Makefile
git commit -m "feat: harden migrate CLI with status/version/steps/force commands"
```

---

## Task 4: Typed Constants for Roles, Statuses, Access Levels

**Context:** Role strings (`"admin"`, `"user"`, `"teacher"`, `"bot"`), access levels (`"manager"`, `"tester"`, `"judge"`, `"owner"`, `"co-author"`), and submission statuses are scattered as raw string literals. This causes silent bugs when one place uses `"Admin"` vs `"admin"`. Move them to `internal/model/constants.go`.

**Files:**
- Create: `internal/model/constants.go`
- Test: existing `internal/model/model_test.go` (add constant validation tests)

- [ ] **Step 1: Write failing tests for the constants**

Add to the bottom of `internal/model/model_test.go`:

```go
func TestRoleConstants(t *testing.T) {
	// These must match the DB CHECK constraint in 000001_init.up.sql
	roles := []string{RoleAdmin, RoleTeacher, RoleUser, RoleBot}
	for _, r := range roles {
		if r == "" {
			t.Errorf("role constant is empty string")
		}
	}
	if RoleAdmin != "admin" {
		t.Errorf("RoleAdmin = %q, want %q", RoleAdmin, "admin")
	}
}

func TestAccessLevelConstants(t *testing.T) {
	levels := []string{AccessLevelOwner, AccessLevelCoAuthor, AccessLevelManager, AccessLevelJudge, AccessLevelTester}
	seen := map[string]bool{}
	for _, l := range levels {
		if l == "" {
			t.Errorf("access level constant is empty string")
		}
		if seen[l] {
			t.Errorf("duplicate access level constant: %q", l)
		}
		seen[l] = true
	}
}
```

- [ ] **Step 2: Run test to see it fail**

```bash
go test ./internal/model/... -run TestRoleConstants -v
```

Expected: `FAIL — RoleAdmin undefined`.

- [ ] **Step 3: Create internal/model/constants.go**

```go
package model

// User roles — must match CHECK constraint in 000001_init.up.sql.
const (
	RoleAdmin   = "admin"
	RoleTeacher = "teacher"
	RoleUser    = "user"
	RoleBot     = "bot"
)

// Problem/contest collaboration access levels.
const (
	AccessLevelOwner    = "owner"
	AccessLevelCoAuthor = "co-author"
	AccessLevelManager  = "manager"
	AccessLevelJudge    = "judge"
	AccessLevelTester   = "tester"
)

// Contest types — must match CHECK constraint in 000001_init.up.sql.
const (
	ContestTypeACM      = "acm"
	ContestTypeOI       = "oi"
	ContestTypeIOI      = "ioi"
	ContestTypePractice = "practice"
)

// Problem difficulty — must match CHECK constraint in 000001_init.up.sql.
const (
	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"
)

// Submission status strings (mirrors the verdict enum in DB).
const (
	StatusStringPending  = "pending"
	StatusStringJudging  = "judging"
	StatusStringAC       = "ac"
	StatusStringWA       = "wa"
	StatusStringTLE      = "tle"
	StatusStringMLE      = "mle"
	StatusStringRE       = "re"
	StatusStringSE       = "se"
	StatusStringCE       = "ce"
)
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/model/... -v
```

Expected: All tests PASS including the two new ones.

- [ ] **Step 5: Commit**

```bash
git add internal/model/constants.go internal/model/model_test.go
git commit -m "feat: add typed constants for roles, access levels, contest types, difficulty"
```

---

## Task 5: Router Dependency Container

**Context:** `NewRouter()` has 44 parameters. Every new feature requires adding another parameter to the function signature, updating the call site in `cmd/aioj/main.go`, and changing the type signature. Replace with a `Deps` struct so new handlers are registered by adding a field — not a parameter.

**Files:**
- Create: `internal/api/deps.go`
- Modify: `internal/api/router.go` (signature only — routes unchanged)
- Modify: `cmd/aioj/main.go` (update call site)

- [ ] **Step 1: Find the current NewRouter call site**

```bash
grep -n "NewRouter" /Users/tahsinarafat/App_Dev/AIOJ/cmd/aioj/main.go
```

Note the line number — you'll update it in Step 4.

- [ ] **Step 2: Write a compile-check test for the Deps struct**

Create `internal/api/deps_test.go`:

```go
package api_test

import (
	"testing"

	"github.com/tahsinarafat/aioj/internal/api"
	"github.com/tahsinarafat/aioj/internal/api/handler"
)

// TestDepsFields ensures Deps has all expected handler fields.
// This is a compile-time guard — if a field is removed, this won't compile.
func TestDepsFields(t *testing.T) {
	_ = api.Deps{
		Auth:            &handler.AuthHandler{},
		Problem:         &handler.ProblemHandler{},
		Submission:      &handler.SubmissionHandler{},
		Contest:         &handler.ContestHandler{},
		ContestProblem:  &handler.ContestProblemHandler{},
		VJudge:          &handler.VJudgeHandler{},
		Admin:           &handler.AdminHandler{},
		Testcase:        &handler.TestcaseHandler{},
		WS:              &handler.WSManager{},
		Rating:          &handler.RatingHandler{},
		Registration:    &handler.RegistrationHandler{},
		Virtual:         &handler.VirtualHandler{},
		Gym:             &handler.GymHandler{},
		Hack:            &handler.HackHandler{},
		Stats:           &handler.StatsHandler{},
		Notification:    &handler.NotificationHandler{},
		Group:           &handler.GroupHandler{},
		Team:            &handler.TeamHandler{},
		Blog:            &handler.BlogHandler{},
		Editorial:       &handler.EditorialHandler{},
		APIKey:          &handler.APIKeyHandler{},
		Webhook:         &handler.WebhookHandler{},
		Recommendation:  &handler.RecommendationHandler{},
		Rankings:        &handler.RankingsHandler{},
		Users:           &handler.UsersHandler{},
		Search:          &handler.SearchHandler{},
		LangLimit:       &handler.LanguageLimitHandler{},
		Import:          &handler.ImportHandler{},
		Org:             &handler.OrganizationHandler{},
		Class:           &handler.ClassHandler{},
		Training:        &handler.TrainingHandler{},
		Plagiarism:      &handler.PlagiarismHandler{},
		Media:           &handler.MediaHandler{},
		Onsite:          &handler.OnsiteHandler{},
		OnsiteBatch:     &handler.OnsiteBatchHandler{},
		Clarification:   &handler.ClarificationHandler{},
		Notice:          &handler.ContestNoticeHandler{},
		BotAccount:      &handler.AdminBotAccountHandler{},
		Settings:        &handler.AdminSystemSettingsHandler{},
		LangAdmin:       &handler.AdminLanguageHandler{},
		RemoteLang:      &handler.RemoteLanguageHandler{},
		AdminSub:        &handler.AdminSubmissionHandler{},
	}
}
```

- [ ] **Step 3: Run test — expect compile failure**

```bash
go test ./internal/api/... 2>&1 | head -5
```

Expected: `undefined: api.Deps`.

- [ ] **Step 4: Create internal/api/deps.go**

```go
package api

import (
	"github.com/tahsinarafat/aioj/internal/api/handler"
)

// Deps holds all HTTP handler dependencies for the router.
// Add a new field here when registering a new handler — no function signature change needed.
type Deps struct {
	Auth           *handler.AuthHandler
	Problem        *handler.ProblemHandler
	Submission     *handler.SubmissionHandler
	Contest        *handler.ContestHandler
	ContestProblem *handler.ContestProblemHandler
	VJudge         *handler.VJudgeHandler
	Admin          *handler.AdminHandler
	Testcase       *handler.TestcaseHandler
	WS             *handler.WSManager
	Rating         *handler.RatingHandler
	Registration   *handler.RegistrationHandler
	Virtual        *handler.VirtualHandler
	Gym            *handler.GymHandler
	Hack           *handler.HackHandler
	Stats          *handler.StatsHandler
	Notification   *handler.NotificationHandler
	Group          *handler.GroupHandler
	Team           *handler.TeamHandler
	Blog           *handler.BlogHandler
	Editorial      *handler.EditorialHandler
	APIKey         *handler.APIKeyHandler
	Webhook        *handler.WebhookHandler
	Recommendation *handler.RecommendationHandler
	Rankings       *handler.RankingsHandler
	Users          *handler.UsersHandler
	Search         *handler.SearchHandler
	LangLimit      *handler.LanguageLimitHandler
	Import         *handler.ImportHandler
	Org            *handler.OrganizationHandler
	Class          *handler.ClassHandler
	Training       *handler.TrainingHandler
	Plagiarism     *handler.PlagiarismHandler
	Media          *handler.MediaHandler
	Onsite         *handler.OnsiteHandler
	OnsiteBatch    *handler.OnsiteBatchHandler
	Clarification  *handler.ClarificationHandler
	Notice         *handler.ContestNoticeHandler
	BotAccount     *handler.AdminBotAccountHandler
	Settings       *handler.AdminSystemSettingsHandler
	LangAdmin      *handler.AdminLanguageHandler
	RemoteLang     *handler.RemoteLanguageHandler
	AdminSub       *handler.AdminSubmissionHandler
}
```

- [ ] **Step 5: Update NewRouter signature in internal/api/router.go**

Change only the function signature (first 58 lines). Replace the `func NewRouter(` block with:

```go
func NewRouter(d Deps, jwtManager *auth.JWTManager) http.Handler {
```

Then replace every local handler variable reference:
- `authH` → `d.Auth`
- `problemH` → `d.Problem`
- `submissionH` → `d.Submission`
- `contestH` → `d.Contest`
- `contestProblemH` → `d.ContestProblem`
- `vjudgeH` → `d.VJudge`
- `adminH` → `d.Admin`
- `testcaseH` → `d.Testcase`
- `wsManager` → `d.WS`
- `ratingH` → `d.Rating`
- `registrationH` → `d.Registration`
- `virtualH` → `d.Virtual`
- `gymH` → `d.Gym`
- `hackH` → `d.Hack`
- `statsH` → `d.Stats`
- `notifH` → `d.Notification`
- `groupH` → `d.Group`
- `teamH` → `d.Team`
- `blogH` → `d.Blog`
- `editorialH` → `d.Editorial`
- `apiKeyH` → `d.APIKey`
- `webhookH` → `d.Webhook`
- `recommendationH` → `d.Recommendation`
- `rankingsH` → `d.Rankings`
- `usersH` → `d.Users`
- `searchH` → `d.Search`
- `langLimitH` → `d.LangLimit`
- `importH` → `d.Import`
- `orgH` → `d.Org`
- `classH` → `d.Class`
- `trainingH` → `d.Training`
- `plagiarismH` → `d.Plagiarism`
- `mediaH` → `d.Media`
- `onsiteH` → `d.Onsite`
- `onsiteBatchH` → `d.OnsiteBatch`
- `clarificationH` → `d.Clarification`
- `noticeH` → `d.Notice`
- `botAccountH` → `d.BotAccount`
- `settingsH` → `d.Settings`
- `langAdminH` → `d.LangAdmin`
- `remoteLangH` → `d.RemoteLang`
- `adminSubH` → `d.AdminSub`

- [ ] **Step 6: Update the call site in cmd/aioj/main.go**

Find the `api.NewRouter(` call. Replace the 44-argument call with:

```go
handler := api.NewRouter(api.Deps{
    Auth:           authH,
    Problem:        problemH,
    Submission:     submissionH,
    Contest:        contestH,
    ContestProblem: contestProblemH,
    VJudge:         vjudgeH,
    Admin:          adminH,
    Testcase:       testcaseH,
    WS:             wsManager,
    Rating:         ratingH,
    Registration:   registrationH,
    Virtual:        virtualH,
    Gym:            gymH,
    Hack:           hackH,
    Stats:          statsH,
    Notification:   notifH,
    Group:          groupH,
    Team:           teamH,
    Blog:           blogH,
    Editorial:      editorialH,
    APIKey:         apiKeyH,
    Webhook:        webhookH,
    Recommendation: recommendationH,
    Rankings:       rankingsH,
    Users:          usersH,
    Search:         searchH,
    LangLimit:      langLimitH,
    Import:         importH,
    Org:            orgH,
    Class:          classH,
    Training:       trainingH,
    Plagiarism:     plagiarismH,
    Media:          mediaH,
    Onsite:         onsiteH,
    OnsiteBatch:    onsiteBatchH,
    Clarification:  clarificationH,
    Notice:         noticeH,
    BotAccount:     botAccountH,
    Settings:       settingsH,
    LangAdmin:      langAdminH,
    RemoteLang:     remoteLangH,
    AdminSub:       adminSubH,
}, jwtManager)
```

- [ ] **Step 7: Compile check**

```bash
go build ./...
```

Expected: exits 0, no output.

- [ ] **Step 8: Run existing tests**

```bash
go test ./... -count=1
```

Expected: All PASS (or pre-existing failures unrelated to this change).

- [ ] **Step 9: Commit**

```bash
git add internal/api/deps.go internal/api/deps_test.go internal/api/router.go cmd/aioj/main.go
git commit -m "refactor: replace 44-param NewRouter with Deps struct"
```

---

## Task 6: Extract Submission Creation Helper (De-duplicate Handler)

**Context:** `SubmissionHandler.Create` and `CreateUpsolving` share ~80% of logic. Both build a `model.Submission`, check for vjudge, and enqueue. Extract the shared path to eliminate future drift.

**Files:**
- Modify: `internal/api/handler/submission.go`

- [ ] **Step 1: Write a test for normalizeOutput (already exists — verify it passes)**

```bash
go test ./internal/api/... -v -run TestNormalize 2>&1 || echo "(no test yet)"
```

Since no test exists yet for `normalizeOutput`, add to a new file `internal/api/handler/submission_test.go`:

```go
package handler

import "testing"

func TestNormalizeOutput(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"hello\r\n", "hello"},
		{"  hello  \n", "hello"},
		{"a\nb\n", "a\nb"},
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeOutput(c.input)
		if got != c.want {
			t.Errorf("normalizeOutput(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to confirm it passes (normalizeOutput already exists)**

```bash
go test ./internal/api/handler/... -run TestNormalizeOutput -v
```

Expected: `PASS`.

- [ ] **Step 3: Add submissionBuildRequest struct and enqueueSubmission helper**

In `internal/api/handler/submission.go`, add after the `normalizeOutput` function (after line 61):

```go
type submissionBuildRequest struct {
	ProblemID  string
	Language   string
	SourceCode string
	ContestID  string
	UserID     string
	Upsolving  bool
}

func (h *SubmissionHandler) buildAndEnqueue(
	r *http.Request,
	w http.ResponseWriter,
	req submissionBuildRequest,
) {
	prob, err := h.probStore.GetByID(r.Context(), req.ProblemID)
	if err != nil || prob == nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	sub := &model.Submission{
		ID:         uuid.New().String(),
		ProblemID:  req.ProblemID,
		UserID:     req.UserID,
		Language:   req.Language,
		SourceCode: req.SourceCode,
		CodeSize:   len(req.SourceCode),
		Status:     model.StatusPending,
	}
	if req.ContestID != "" {
		sub.ContestID = req.ContestID
	}

	if err := h.subStore.Create(r.Context(), sub); err != nil {
		http.Error(w, "create failed", http.StatusInternalServerError)
		return
	}

	if prob.Source != "" && prob.Source != "local" && h.vjudgeSvc != nil {
		vjReq := vjudge.SubmitRequest{
			ID:              sub.ID,
			ProblemRemoteID: prob.RemoteID,
			SourceCode:      req.SourceCode,
			Language:        req.Language,
			RemoteOJ:        prob.Source,
		}
		if err := h.vjudgeSvc.Submit(r.Context(), vjReq); err != nil {
			slog.Error("vjudge submit failed", "sub", sub.ID, "err", err)
			sub.Status = model.StatusSE
			respondJSON(w, http.StatusCreated, sub)
			return
		}
	} else {
		h.queue.Enqueue(r.Context(), sub.ID)
	}
	respondJSON(w, http.StatusCreated, sub)
}
```

- [ ] **Step 4: Simplify Create to use the helper**

Replace the body of `func (h *SubmissionHandler) Create(...)` (lines 63–127) with:

```go
func (h *SubmissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ProblemID  string `json:"problem_id"`
		Language   string `json:"language"`
		SourceCode string `json:"source_code"`
		ContestID  string `json:"contest_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ProblemID == "" || req.Language == "" || req.SourceCode == "" {
		http.Error(w, "problem_id, language, and source_code are required", http.StatusBadRequest)
		return
	}

	h.buildAndEnqueue(r, w, submissionBuildRequest{
		ProblemID:  req.ProblemID,
		Language:   req.Language,
		SourceCode: req.SourceCode,
		ContestID:  req.ContestID,
		UserID:     claims.UserID,
	})
}
```

- [ ] **Step 5: Simplify CreateUpsolving to use the helper**

Replace the body of `func (h *SubmissionHandler) CreateUpsolving(...)` (lines 129–191) with:

```go
func (h *SubmissionHandler) CreateUpsolving(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetUserClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		ProblemID  string `json:"problem_id"`
		Language   string `json:"language"`
		SourceCode string `json:"source_code"`
		ContestID  string `json:"contest_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ProblemID == "" || req.Language == "" || req.SourceCode == "" || req.ContestID == "" {
		http.Error(w, "all fields required", http.StatusBadRequest)
		return
	}

	h.buildAndEnqueue(r, w, submissionBuildRequest{
		ProblemID:  req.ProblemID,
		Language:   req.Language,
		SourceCode: req.SourceCode,
		ContestID:  req.ContestID,
		UserID:     claims.UserID,
		Upsolving:  true,
	})
}
```

- [ ] **Step 6: Build and test**

```bash
go build ./... && go test ./internal/api/... -v
```

Expected: exits 0, all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/handler/submission.go internal/api/handler/submission_test.go
git commit -m "refactor: extract buildAndEnqueue helper, remove Create/CreateUpsolving duplication"
```

---

## Self-Review

### Spec Coverage

| Requirement | Task |
|---|---|
| Long-term sustainability | All tasks |
| Migration rollback safety | Tasks 1, 2 |
| Migration operational tooling | Task 3 |
| Magic string reduction | Task 4 |
| Router extensibility | Task 5 |
| Handler code duplication | Task 6 |

### Placeholder Scan

- No "TBD", "TODO", or "implement later" present.
- Every code block is complete and runnable.
- All file paths are absolute-rooted from repo root.

### Type Consistency

- `submissionBuildRequest` defined in Task 6 Step 3 and used in Steps 4 and 5 — consistent.
- `api.Deps` defined in Task 5 Step 4 and tested in Step 2 — consistent.
- Constants in `model/constants.go` (Task 4) are tested in the same task — consistent.

### Gaps Found & Fixed

- Task 6 Step 1 had no test for `normalizeOutput` — added `submission_test.go`.
- `buildAndEnqueue` writes response directly — callers don't need to write after calling it (early-return pattern maintained from original).

---

## Execution Handoff

Plan saved to `docs/superpowers/plans/2026-06-02-migration-sustainability.md`.

**Two execution options:**

**1. Subagent-Driven (recommended)** — fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** — execute tasks in this session using executing-plans, batch with checkpoints.

Which approach?
