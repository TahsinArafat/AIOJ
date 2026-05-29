# AIOJ Project Context — For AI Agents

> Read this first before working on any code. It gives you everything you need.

---

## What is AIOJ?

A full-featured competitive programming platform (Codeforces alternative). Monolith Go backend, React frontend, PostgreSQL, Docker sandbox.

---

## Quick Start

```bash
docker compose up -d
make migrate-up
```

**URLs:**
- Frontend: http://localhost (React SPA via nginx)
- Backend: http://localhost:8080 (Go, Chi router)
- Judge: http://localhost:5050 (go-judge sandbox)
- Postgres: localhost:5432 (user: aioj, db: aioj)

**Default admin:** admin / admin_secret

---

## Architecture

```
Browser -> nginx(:80) -> React SPA (Vite + TypeScript + Tailwind)
                     -> /api/* proxy -> Go backend(:8080)
                                        |-- PostgreSQL(:5432)
                                        +-- go-judge(:5050) sandbox
```

**Key packages:**
- `cmd/aioj/main.go` — Entry point, wires everything
- `internal/api/router.go` — All routes defined here
- `internal/api/handler/` — HTTP handlers (21 files)
- `internal/store/postgres/` — Database implementations (14 files)
- `internal/store/interfaces.go` — All store interfaces
- `internal/model/` — Shared types and constants
- `internal/judge/worker.go` — Compilation + test case execution
- `internal/judge/executor/` — HTTP client to go-judge sandbox
- `web/src/` — React frontend source

**Database migrations** are at `internal/store/migrations/` (numbered .up.sql / .down.sql)

**Language configs** are at `lang/*.yaml` (C++, Python, Java, Rust, Node.js, etc.)

---

## Judging Pipeline

1. User submits code via `POST /api/submissions`
2. Submission saved with status "pending"
3. WorkerPool picks it up, compiles via go-judge sandbox
4. For each test case, runs the binary in sandbox with input piped
5. Checks output against expected using checker (exact, lines, float)
6. Updates submission with verdict (AC/WA/TLE/MLE/RE/CE/SE)

**Important:** Compilers (g++, python3, java, rust, nodejs) are installed in the **backend** container (see Dockerfile). go-judge only does isolated execution, not compilation.

---

## Implemented Features (status as of 2026-05-29 — Superiority Roadmap COMPLETE)

| Phase | Plans | Status |
|-------|-------|--------|
| Phase 1 - Core | 01-04 | Done |
| Phase 2 - Contests | 05-08 | Done |
| Phase 3 - Engagement | 09-11 | Done |
| Phase 4 - Community | 12-14 | Done |
| Phase 5 - Content | 15-16 | Done |
| Phase 6 - Platform | 17-19 | Done |
| Phase 7 - Polish | 20-23 | Done |
| **Superiority Phase 1** | Judging Engine v2 (interactive, submit-answer, subtask, 8 checkers, per-lang limits) | Done |
| **Superiority Phase 2** | Problem Ecosystem (FPS import/export, ZIP test case upload) | Done |
| **Superiority Phase 3** | Contest Depth (pluggable format registry: ACM/OI/IOI/AtCoder/CF) | Done |
| **Superiority Phase 4** | Community Platform (orgs, classes, training plans, auto-progress) | Done |
| **Superiority Phase 5** | Quality Assurance (LCS plagiarism detection, admin dashboard) | Done |
| **Superiority Phase 6** | Scale & Reliability (Redis queue, distributed judge worker, circuit breaker, read replicas) | Done |
| **Superiority Phase 7** | Polish (Prometheus /metrics, Grafana dashboards, alerts) | Done |

| Phase | Plans | Status |
|-------|-------|--------|
| Phase 1 - Core | 01-04 | Done |
| Phase 2 - Contests | 05-08 | Done |
| Phase 3 - Engagement | 09-11 | Done |
| Phase 4 - Community | 12-14 | Done |
| Phase 5 - Content | 15-16 | Done |
| Phase 6 - Platform | 17-19 | Done |
| Phase 7 - Polish | 20-23 | Done |

**01** Rating System - Elo calculator, color badges (Newbie-LGM)
**02** Contest Registration - Register/unregister, deadlines, max participants
**03** Division System - Div 1/2/3/4, eligibility checks, filtering
**04** Problem Filtering - Tags, difficulty, search
**05** Virtual Contests - Past contest simulation with timer
**06** Gym/Training - Community contests, difficulty ratings, categories
**07** Educational Rounds - Hack phases, editorial config
**08** Upsolving - Post-contest problem solving
**09** Hacking System - Counter-test cases, hack scoring
**10** Problem Statistics - Language distribution, acceptance rates
**11** Notifications - Bell icon with real-time polling
**12** Groups - Create/join, members, group contests
**13** Teams - Team rating, members, join/leave
**14** Blog/Discussions - Posts, comments, voting
**15** Editorials - Official/community solutions with code
**16** Problem Recommendations - Personalized progression and weak topic analysis
**17** Public API - API key management
**18** Rate Limiting - Infrastructure ready
**19** Webhooks - Event delivery with secret signing
**20** i18n - English + Bengali (react-i18next)
**21** PWA - Manifest, service worker, offline page
**22** Performance - Database indexes
**23** Monitoring - Prometheus config, Grafana dashboards, alerts

---

## Common Tasks

### Add a new API endpoint
1. Define handler methods in `internal/api/handler/<name>.go`
2. Add routes in `internal/api/router.go` (add parameter + route group)
3. Wire constructor in `cmd/aioj/main.go`

### Add a new database table
1. Create migration files in `internal/store/migrations/` (numbered)
2. Add model in `internal/model/<name>.go`
3. Add store interface in `internal/store/interfaces.go`
4. Implement store in `internal/store/postgres/<name>.go`

### Add a new frontend page
1. Create `web/src/pages/<PageName>.tsx`
2. Import and add route in `web/src/App.tsx`
3. Add API calls in `web/src/lib/api.ts`
4. Add navigation link in the Navbar in `App.tsx`

### Run tests
```bash
go test ./... -count=1             # Backend (10 packages, 145+ tests)
cd web && npm run build            # Frontend TypeScript check
```

### Rebuild after code changes
```bash
docker compose build backend       # Rebuild backend
docker compose build frontend      # Rebuild frontend
docker compose up -d               # Deploy
```

### Fix dirty database migration
```bash
docker compose exec postgres psql -U aioj -d aioj \
  -c "UPDATE schema_migrations SET version = 15, dirty = false"
```

---

## Known Bugs & Gotchas

1. **Problem visibility**: New problems were being created with `visible=false`. Fixed — now defaults to `visible=true`. If setter panel is empty, run: `UPDATE problems SET visible = true`

2. **go-judge response format**: The sandbox returns `[]CmdResult` array directly, not wrapped in an `ExecResponse` object. The `Files` field is `map[string]string` (content), not `map[string]CmdFile`.

3. **Migration 16**: Uses `CREATE INDEX CONCURRENTLY` which fails inside transactions. Changed to `CREATE INDEX IF NOT EXISTS`. If dirty, force version 15.

4. **`virtual.GetStatus()`**: Was using `time.Until()` ignoring the `now` parameter. Fixed to use `endsAt.Sub(now)`.

5. **Compiler not found**: go-judge container is distroless. Compilers MUST be installed in the backend container (Dockerfile line 12).

6. **Sample cases**: The API accepts `sample_cases: null` — this is fine. The problem store stores them as JSONB.

---

## File Index

### Backend (Go)
```
cmd/aioj/main.go                          Entry point
internal/api/router.go                    All routes
internal/api/handler/*.go                 21 handlers
internal/api/middleware/*.go              Auth, role, logging
internal/model/*.go                       18 model files
internal/store/interfaces.go             All interfaces
internal/store/postgres/*.go             Database implementations
internal/store/migrations/*.sql          Migrations (000001-000016)
internal/judge/worker.go                 Judging engine
internal/judge/executor/executor.go      go-judge HTTP client
internal/judge/checker/checker.go        Output checkers
internal/rating/service.go              Rating calculation
internal/virtual/service.go             Virtual contest logic
internal/hack/service.go               Hack validation
internal/notification/service.go        Notification dispatch
internal/auth/*.go                      JWT + password hashing
lang/*.yaml                             Language compiler configs
```

### Frontend (React)
```
web/src/App.tsx                  Router + Navbar
web/src/lib/api.ts               API client (all endpoints)
web/src/lib/rating.ts            Rating color utilities
web/src/lib/divisions.ts         Division utilities
web/src/pages/*.tsx              20+ page components
web/src/components/*.tsx         8 reusable components
web/src/i18n/                    i18n config + EN/BN locales
```

### Infrastructure
```
Dockerfile                       Backend build + run
web/Dockerfile                   Frontend build
docker-compose.yml               All services
deploy/prometheus/               Monitoring config
deploy/grafana/                  Dashboards
docs/superpowers/plans/          Implementation plans
docs/analysis/                   Gap analysis
```

---

## Testing Status

| Package | Tests | Status |
|---------|-------|--------|
| internal/model/ | 66 | PASS |
| internal/auth/ | 18 | PASS |
| internal/rating/ | 6 | PASS |
| internal/virtual/ | 15 | PASS |
| internal/hack/ | 5 | PASS |
| internal/middleware/ | 13 | PASS |
| internal/notification/ | 15 | PASS |
| internal/judge/ | 5 | PASS |
| internal/queue/ | 2 | PASS |
| **Total** | **145+** | **ALL PASS** |
