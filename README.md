<div align="center">

# AIOJ

**A full-featured, self-hostable competitive programming platform.**

Monolith Go backend · React SPA · PostgreSQL · Docker-isolated code execution

[![Backend tests](https://github.com/TahsinArafat/AIOJ/actions/workflows/backend-test.yml/badge.svg)](https://github.com/TahsinArafat/AIOJ/actions/workflows/backend-test.yml)
[![Frontend build](https://github.com/TahsinArafat/AIOJ/actions/workflows/frontend-build.yml/badge.svg)](https://github.com/TahsinArafat/AIOJ/actions/workflows/frontend-build.yml)
[![Lint](https://github.com/TahsinArafat/AIOJ/actions/workflows/lint.yml/badge.svg)](https://github.com/TahsinArafat/AIOJ/actions/workflows/lint.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](#license)

</div>

---

## Table of Contents

- [What is AIOJ?](#what-is-aioj)
- [Feature Highlights](#feature-highlights)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Quick Start (Docker)](#quick-start-docker)
- [Local Development](#local-development)
- [Configuration](#configuration)
- [Project Layout](#project-layout)
- [How Judging Works](#how-judging-works)
- [Virtual Judge (Remote Platforms)](#virtual-judge-remote-platforms)
- [API & Integrations](#api--integrations)
- [Testing & CI](#testing--ci)
- [Common Tasks](#common-tasks)
- [Production Deployment](#production-deployment)
- [Troubleshooting](#troubleshooting)
- [Documentation](#documentation)
- [License](#license)

---

## What is AIOJ?

AIOJ is a complete online judge you can run yourself — a Codeforces-style platform for hosting problems, running contests, judging submissions, and building a community. It is designed to be **lightweight to operate** (a single Go binary plus Postgres and a sandbox) while still covering the depth expected of a modern OJ: virtual contests, hacking, editorial workflows, organizations and classes, plagiarism detection, and remote-platform (virtual judge) submissions.

**Who it's for:** university courses, clubs, and anyone who wants a private competitive-programming judge without the operational weight of a distributed system.

---

## Feature Highlights

**Judging**
- Secure, cgroup-isolated execution via [`criyle/go-judge`](https://github.com/criyle/go-judge).
- 12 pre-configured languages (C/C++/Clang/GCC 32/64, Python, PyPy, Java, Rust, Node.js, JavaScript, C#) declared in `lang/*.yaml`.
- Rich judging engine: subtasks, interactive problems, submit-answer problems, per-language limits, and 8 output checkers (byte-identical, sorted, unordered, float absolute/relative, and more).
- Fully asynchronous queue backed by Redis (falls back to an in-memory queue when Redis is unset).

**Contests**
- Pluggable scoring-format registry: **ACM / OI / IOI / AtCoder / Codeforces**.
- Scoreboards with freeze times, virtual contests, gym/training contests, and upsolving.
- Hacking phase with counter-test cases and hack scoring.
- Divisions (Div 1–4) with eligibility checks.

**Problems & Content**
- Problem setter workspace with a visual statement editor (TipTap), test-case upload, and ZIP import.
- FPS (Free Polygon Standard) import/export.
- Blog/discussions, editorials (official + community), clarification & contest notices.
- Problem i18n (English + Bengali via `react-i18next`), KaTeX math, tags, difficulty, and recommendations.

**Community & Platform**
- Organizations, classes, and training plans with automatic progress tracking.
- Groups and teams, friendships, achievements, and an activity feed.
- Notifications (real-time over WebSocket), public profiles, rating history with Elo and color badges.
- Public REST API with API-key management, webhooks with secret signing, rate limiting.
- OAuth sign-in (GitHub & Google), 2FA (TOTP + backup codes), email verification, and password reset.
- Observability: Prometheus `/metrics`, Grafana dashboards, alert rules, and Sentry error tracking.
- SEO/PWA: dynamic `sitemap.xml`, Atom feeds, manifest, and service worker.

**Virtual Judge**
- Submit to remote platforms through integrated bots: **Codeforces, AtCoder, CSES, Toph, QOJ**.
- Cloudflare-bypass submit services for Codeforces and AtCoder.

---

## Architecture

AIOJ is a monolith with a clear separation between the API server and dedicated judge workers, so judging scales independently from request handling.

```
                         ┌─────────────────────────────────────────┐
   Browser ──▶ nginx:80  │  React SPA (Vite + TS + Tailwind)        │
                         └─────────────────────────────────────────┘
                                        │ /api/* proxy
                                        ▼
                         ┌─────────────────────────────────────────┐
                         │  Go API server (:8080)  — Chi router     │
                         │  handlers · middleware · stores          │
                         └───────┬───────────────┬─────────────────┘
                                 │               │
                    ┌────────────▼───┐     ┌─────▼───────────────┐
                    │ PostgreSQL:5432│     │ Redis :6379 (queue) │
                    └────────────────┘     └─────┬───────────────┘
                                                 │ claim
                                       ┌─────────▼────────────┐
                                       │ judge-worker process │
                                       │ (compile + verify)   │
                                       └─────────┬────────────┘
                                                 │ HTTP
                                       ┌─────────▼────────────┐
                                       │ go-judge sandbox:5050│
                                       │ (cgroup isolated)    │
                                       └──────────────────────┘
```

**Key design decisions**

| Decision | Why |
|----------|-----|
| **Monolith + separate worker mode** | One binary, `--mode=server\|judge-worker`. Only workers consume the judge queue, so the API never silently becomes a second decider. |
| **Compilers in the backend image** | `go-judge` is distroless and only executes; compilation (g++, python3, java, rustc, node) happens in the backend/judge-worker image. |
| **Redis-backed queue** | Enables multiple `judge-worker` replicas. Includes claim-token fencing and a reclaimer for stale claims. |
| **Pluggable contest formats** | `internal/contest/format/` registry — adding a scoring rule means adding one package, not editing the core. |
| **Store interfaces** | `internal/store/interfaces.go` defines contracts; `internal/store/postgres/` implements them, keeping handlers testable. |

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.26, [Chi](https://github.com/go-chi/chi) router, `log/slog`, `golang-migrate` |
| Database | PostgreSQL 18 |
| Queue / cache | Redis 7 (`go-redis`), in-memory fallback |
| Sandbox | `criyle/go-judge` (privileged, host cgroup namespace) |
| Frontend | React 19, TypeScript 6, Vite 8, Tailwind CSS 4, React Router 7 |
| Editor | Monaco Editor; TipTap for rich-text statements |
| Testing | Go `testing`, Vitest + Testing Library (unit), Playwright (E2E) |
| Observability | Prometheus, Grafana, Sentry |
| Auth | JWT (access + refresh), TOTP 2FA, OAuth (GitHub/Google) |

---

## Quick Start (Docker)

### Prerequisites
- Docker and Docker Compose.

### 1. Configure environment

```bash
cp .env.example .env
```

Edit `.env` and set strong secrets:

```env
DB_PASSWORD=<long-random-password>
JWT_SECRET=<long-random-string>
```

Generate a secret with `openssl rand -hex 32`.

`JWT_SECRET` is mandatory and validated at startup: the backend refuses to boot
with an empty, shorter-than-32-byte, or published-default secret, because anyone
who knows the signing key can forge an admin token.

`AIOJ_ENV` selects the profile. It defaults to `production`; set
`AIOJ_ENV=development` for a local checkout (this enables the
`/api/dev/mail` inspector and only warns about weak secrets).

### 2. Start the stack

```bash
docker compose up -d
```

Wait a few seconds for PostgreSQL and the judge sandbox to initialize. The backend applies database migrations automatically on startup.

### 3. Open the app

- Application: [http://localhost](http://localhost)

### Default admin credentials

| Username | Password |
|----------|----------|
| `admin`  | `admin_secret` |

> **Change the admin password immediately** after first login (Profile → Change Password).

### Service ports (local)

| Service | Port | Purpose |
|---------|------|---------|
| Frontend (nginx) | 80 | React SPA |
| Backend (Go) | 8080 | REST API + WebSocket |
| Prometheus | 127.0.0.1:9090 | Metrics |
| Grafana | 127.0.0.1:3000 | Dashboards (login required) |

PostgreSQL (5432), Redis (6379) and go-judge (5050) are **not published** by
`docker-compose.yml`: they are reachable only from other containers on the
compose network. go-judge in particular runs privileged and accepts arbitrary
argv, and Redis holds the judge queue — neither belongs on a host port. Run
`docker compose exec postgres psql -U aioj` for database access.

---

## Local Development

### Backend

```bash
# Start only the dependencies
docker compose up -d postgres judge redis

# Run the API server with hot reload via go run
make run

# ...or run a judge worker in a second terminal
go run ./cmd/aioj --mode=judge-worker
```

The server reads `config.yaml` (`make run` uses the repo default). Point it at the Dockerized Postgres/Redis with the values in `config.yaml.example`.

### Frontend

```bash
cd web
npm install
npm run dev        # Vite dev server on :5173
```

The dev server proxies `/api` to the backend. Build a production bundle with `npm run build`.

### Database migrations

```bash
make migrate-up         # apply all pending migrations
make migrate-down       # roll back one
make migrate-status     # show current state
make migrate-version    # print current version
make migrate-force V=71 # force version (recover from a dirty state)
```

Migrations live in `internal/store/migrations/` as numbered `.up.sql` / `.down.sql` pairs. The API server also auto-applies them on boot (`cmd/aioj/main.go`).

---

## Configuration

Configuration is layered: **`config.yaml`** provides defaults, and environment variables override sensitive values (the intended pattern for containers).

| Env var | Purpose | Default |
|---------|---------|---------|
| `DB_HOST` / `DB_PORT` / `DB_NAME` / `DB_USER` / `DB_PASSWORD` | PostgreSQL connection | `localhost:5432 / aioj` |
| `AIOJ_ENV` | `production` \| `development` (dev enables `/api/dev/mail` and relaxes secret checks) | `production` |
| `JWT_SECRET` | Signs access/refresh tokens — **required**, ≥32 bytes, no published defaults | none |
| `TRUSTED_PROXIES` | CIDRs allowed to set `X-Forwarded-For`; empty disables header trust | loopback + RFC1918 |
| `CSRF_SECRET` | CSRF token signing | falls back to JWT secret |
| `JUDGE_ENDPOINT` | go-judge base URL | `http://localhost:5050` |
| `JUDGE_CONCURRENCY` | Parallel judge workers | `4` |
| `REDIS_URL` | Redis address; empty → in-memory queue | → queue |
| `LANG_DIR` | Directory of `lang/*.yaml` | `./lang` |
| `MAIL_DRIVER` | `smtp` \| `catcher` \| `noop` | `catcher` |
| `MAIL_PUBLIC_URL` / `PUBLIC_ORIGIN` | Public base URL for links, sitemap, feeds | `http://localhost:8081` |
| `GITHUB_CLIENT_ID` / `GOOGLE_CLIENT_ID` (+ secrets) | Enable OAuth providers | disabled |
| `CF_SUBMIT_URL` | Codeforces submit service | `http://host.docker.internal:8003` |
| `ATCODER_SUBMIT_URL` | AtCoder submit service | `http://host.docker.internal:8004` |
| `SENTRY_DSN` | Error tracking (optional) | disabled |

See `config.yaml.example` for the full annotated structure.

---

## Project Layout

```
cmd/
  aioj/          Main binary — API server + judge worker (--mode)
  migrate/       Standalone migration tool
  seed/          Database seeder (demo data / admin)
  aioj-cli/      Admin CLI
  vjudge-test/   Manual vjudge bot harness

internal/
  api/
    router.go        All HTTP routes (Chi)
    deps.go          Handler dependency wiring
    handler/         HTTP handlers (problems, contests, submissions, …)
    middleware/      Auth, role, CSRF, rate limit, logging, Sentry recover
  auth/              JWT, password hashing, TOTP
  config/            config.yaml + env loading
  contest/format/    ACM / OI / IOI / AtCoder / Codeforces scoring registry
  judge/
    worker.go        Compilation + test-case orchestration
    executor/        HTTP client to go-judge
    checker/         Output checkers
    compiler/        Language → compile command resolution
    runner/          Batch + interactive runners, scorer
  store/
    interfaces.go    Store contracts
    postgres/        PostgreSQL implementations
    migrations/      Numbered .up.sql / .down.sql
  model/             Shared domain types
  rating/            Elo calculation
  virtual/           Virtual contest logic
  hack/              Hack validation + scoring
  plagiarism/        LCS engine + MOSS integration
  vjudge/            Remote-platform bots & parsers
  mail/              SMTP / MailCatcher senders + templates
  oauth/             GitHub & Google providers
  queue/             Redis + in-memory judge queues
  fps/               Free Polygon Standard import/export
  generate/          AI-assisted problem generation
  observability/     Sentry initialization

web/                 React SPA (Vite)
  src/
    pages/           Route components
    components/      Reusable UI (Navbar, Editor, RatingGraph, …)
    lib/             API client, utilities
    i18n/            EN + BN locales
    test/            Vitest setup & fixtures
  e2e/               Playwright specs

lang/*.yaml          Language compiler/run configurations
deploy/              Caddyfile, nginx, prometheus, grafana, status page
docs/                OpenAPI spec, legal, runbooks, plans
```

---

## How Judging Works

```
POST /api/submissions
        │
        ▼
  status = pending  ──▶  JudgeQueue (Redis list, or in-memory)
                                │
                                ▼
                     judge-worker claims the job
                     (claim token + fencing, stale-claim reclaimer)
                                │
                                ▼
             compile in backend image ──▶ run each test case
                                          in go-judge sandbox
                                │
                                ▼
             checker verdicts (AC / WA / TLE / MLE / RE / CE / SE)
                                │
                                ▼
             persist verdict ──▶ WebSocket push to the browser
```

1. The submission is stored with `status = pending` and pushed onto the judge queue.
2. A **judge worker** claims it with a fencing token. Only `--mode=judge-worker` processes consume the queue.
3. The worker **compiles** the source in the backend image (compilers are *not* in the distroless judge container).
4. Each test case is executed in the **go-judge sandbox** with input piped and limits enforced by cgroup.
5. Output is verified by the configured **checker** (exact, lines, float, sorted, unordered, …).
6. The verdict is persisted and pushed to the client over **WebSocket** in real time.

**Languages** are declared as data in `lang/*.yaml`, so adding a language rarely requires Go changes.

---

## Virtual Judge (Remote Platforms)

AIOJ can submit code to remote judges on behalf of bot accounts. Bots live in `internal/vjudge/` and are registered at startup for the platforms **Codeforces, AtCoder, CSES, Toph, and QOJ**.

- **Codeforces** and **AtCoder** use dedicated submit microservices (`deploy/cf-submit/`, `deploy/atcoder-submit/`) that drive a real browser to clear Cloudflare.
- Bot credentials are managed under **Admin Panel → Bot Accounts**, with automatic disable after consecutive failures.

> **Cloudflare note:** Codeforces blocks datacenter IPs. On a VPS you **must** route through a residential proxy (`CF_PROXY`) or the login form will never render. See the [cf-submit troubleshooting](#troubleshooting) below.

---

## API & Integrations

- **REST API** — served under `/api/*`. An OpenAPI spec lives at `docs/openapi/openapi.yaml`; regenerate TypeScript types with `make openapi`.
- **API keys** — users create keys in Settings → API; requests authenticate with the key header.
- **Webhooks** — event delivery with HMAC secret signing, configurable per user/org.
- **WebSocket** — `/api/ws` pushes submission verdicts and notifications.
- **Atom feeds** — `/feed/problems.atom`, `/feed/contests.atom`, `/feed/comments.atom`, `/feed/blog.atom`.
- **Sitemap** — `/sitemap.xml`, built from the same visibility rules the public APIs enforce.
- **Metrics** — `/metrics` (Prometheus).
- **Status page** — `/status`, a static page polling `/api/health`.

---

## Testing & CI

### Run tests locally

```bash
# Backend (uses AIOJ_TEST_DSN, defaults to local Postgres)
make test

# Frontend unit tests (Vitest)
make test-frontend

# Lint (gofmt check + go vet + golangci-lint + eslint) — mirrors CI
make lint

# End-to-end (Playwright) — requires the sim stack
make sim-up
make e2e
```

### CI workflows (`.github/workflows/`)

| Workflow | What it checks |
|----------|---------------|
| `backend-test.yml` | `go test ./cmd/... ./internal/...` + build against a live Postgres |
| `frontend-build.yml` | TypeScript typecheck + Vite build + Vitest |
| `lint.yml` | `gofmt -l`, `go vet`, `golangci-lint`, `eslint` |
| `docker-image.yml` | Builds and publishes container images |
| `deploy-staging.yml` | Staging deployment |

---

## Common Tasks

### Add a new API endpoint
1. Add handler methods in `internal/api/handler/<name>.go`.
2. Add a field to `Deps` in `internal/api/deps.go` and wire routes in `internal/api/router.go`.
3. Construct the handler in `cmd/aioj/main.go`.

### Add a new database table
1. Create `NNNNNN_<name>.up.sql` / `.down.sql` in `internal/store/migrations/`.
2. Add the model in `internal/model/<name>.go`.
3. Add the store interface in `internal/store/interfaces.go`.
4. Implement it in `internal/store/postgres/<name>.go`.

### Add a new frontend page
1. Create `web/src/pages/<PageName>.tsx`.
2. Register the route in `web/src/App.tsx`.
3. Add API calls in `web/src/lib/api.ts` and a nav link in the Navbar.

### Add a language
1. Drop a new YAML into `lang/` (copy an existing file as a template).
2. Ensure the compiler/runtime is present in the `Dockerfile`.
3. Restart the backend — languages are auto-discovered from the directory.

### Rebuild after code changes

```bash
docker compose build backend      # or: frontend / judge-worker
docker compose up -d
```

---

## Production Deployment

The full, beginner-friendly VPS guide (VPS + domain + HTTPS with Caddy or Nginx, firewalls, visual management panels, and bot setup) is maintained in **[`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md)**. The short version:

1. **Provision** an Ubuntu VPS and a domain; point an `A` record at the server IP.
2. **Harden** — allow only ports 22/80/443; never expose 5432/6379/5050/8080.
3. **Clone** the repo, copy `.env.example` → `.env`, and set strong secrets.
4. **Bring up core services**, run migrations, then enable a reverse proxy:

   ```bash
   docker compose up -d postgres judge redis backend judge-worker frontend
   make migrate-up
   docker compose --profile production up -d   # starts Caddy (edit deploy/Caddyfile first)
   ```

   Caddy obtains and renews Let's Encrypt certificates automatically. To use Nginx instead, see `deploy/nginx/aioj.conf`.

5. **Log in and immediately change the admin password.**

> **Scale-out:** run multiple `judge-worker` replicas with `REDIS_URL` set — the queue enforces claim fencing so workers never double-judge a submission.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `Site can't be reached` after deploy | DNS not propagated | Wait 5–30 min; verify with `nslookup yourdomain.com`. |
| `502 Bad Gateway` | Backend container down | `docker compose ps` — all services should be `Up`. |
| Migrations fail on boot | DB not ready | Wait longer; check `docker compose logs postgres`. |
| Migration stuck **dirty** | A failed migration left partial state | `docker compose exec postgres psql -U aioj -d aioj -c "UPDATE schema_migrations SET version=<n>, dirty=false"`. |
| Judge always `CE` | Compiler missing in backend image | Compilers live in the **backend** image, not distroless go-judge. Rebuild `backend`. |
| Submissions stuck `pending` | No judge worker consuming the queue | Ensure a `judge-worker` (or `--mode=judge-worker`) process is running and shares `REDIS_URL`. |
| CF submission → `SE` / `System Error` | cf-submit unreachable, bot locked, or Cloudflare blocked | Check `curl localhost:8003/health` → `"proxy":true`. Reset a locked bot: `UPDATE bot_accounts SET consecutive_failures=0, status='active'`. |
| `dial tcp ...:8003: i/o timeout` | UFW blocking Docker→host traffic | `sudo ufw allow from 172.16.0.0/12 to any port 8003 proto tcp && sudo ufw reload`. |
| `login form not found. CF blocked: True` | Proxy not working | Codeforces sees your datacenter IP — configure a residential `CF_PROXY`. |
| `lookup host.docker.internal: no such host` | Missing `extra_hosts` | Ensure `extra_hosts: ["host.docker.internal:host-gateway"]` is set (already in `docker-compose.yml`). |
| `cgroup path is empty` (judge restarts) | Docker private cgroup namespace | The judge service requires `cgroup: host` + `privileged: true` (already configured). |

For cf-submit deep debugging (screenshots, manual browser tests, systemd setup), see `deploy/cf-submit/`.

---

## Documentation

| Document | Contents |
|----------|----------|
| [`AI_CONTEXT.md`](AI_CONTEXT.md) | Architecture, common tasks, and gotchas for contributors/AI agents |
| [`USER_GUIDE.md`](USER_GUIDE.md) | End-user guide (solving, contests, profile) |
| [`COMPETITIVE_OJ_FEATURES.md`](COMPETITIVE_OJ_FEATURES.md) | Feature parity notes vs. other judges |
| [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) | Full VPS + domain + HTTPS deployment guide and bot setup |
| [`docs/openapi/openapi.yaml`](docs/openapi/openapi.yaml) | REST API specification |
| [`docs/runbooks/`](docs/runbooks/) | Operational runbooks (disaster recovery, domain policy, multi-region) |
| [`docs/superpowers/plans/`](docs/superpowers/plans/) | Historical implementation plans |

---

## License

Released under the [MIT License](LICENSE).

---

<div align="center">

Built for the competitive programming community.

</div>
