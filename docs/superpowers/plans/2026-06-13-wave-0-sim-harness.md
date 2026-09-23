# Wave 0 — Sim Harness (Playwright + Docker Compose) Implementation Plan

**Goal:** Stand up the simulation harness for the Full-Scale Completion program: a reusable
Docker Compose `sim` profile, an API-driven seeder, and a Playwright suite whose first tests
walk health → register → login → create problem + testcase → submit C++ → **AC**.

**Architecture:** One command boots the whole product exactly as it exists on `main`
(`docker compose -f docker-compose.yml -f docker-compose.sim.yml --profile sim up -d --build`).
The `sim` overlay only *adds* `mailhog` and *rebinds* `frontend` to **8081**; it never edits the
base compose. There is **no supervisor service, no new Dockerfile, no new Go binary mode** —
seeding is a plain Go program (`cmd/seed`) run as a one-shot `docker compose run --rm`.

Verified against live `main` (2026-06-13):

| Fact | Value |
|---|---|
| Base compose | `docker-compose.yml` (services: postgres, judge, redis, backend, judge-worker, frontend, prometheus, grafana, cf-bypass, cf-submit, atcoder-submit, caddy[profile=production]) |
| Override on host | `docker-compose.override.yml` (only unpublishes postgres 5432) |
| Frontend image | `web/Dockerfile` — nginx serving the Vite **prod build** on :80 |
| Backend | `Dockerfile` → `/app/aioj`, `--mode=server` (default) / `--mode=judge-worker` |
| Health | `GET /api/health` → `{"status":"ok"}` (rate-limit exempt) |
| Register | `POST /api/auth/register` `{username,email,password}` → 201 + `access_token`/`refresh_token`; role hardcoded `"user"` |
| Login | `POST /api/auth/login` `{username,password}` → 200 + tokens |
| Promote to admin | `PUT /api/admin/users/{id}/role` `{"role":"admin"}` — **requires an existing admin**, so the seed bootstraps the *first* admin directly in the DB |
| Problem create | `POST /api/problems` (auth) `model.CreateProblemRequest` — `slug,title,description,time_limit,memory_limit,difficulty,sample_cases,visible` |
| Testcase upload | `POST /api/problems/{slug}/testcases` multipart field `file` (zip of `.in`/`.out`), admin/owner/co_author only |
| Submit | `POST /api/submissions` (auth) `model.SubmitRequest` `{problem_id,language,source_code,contest_id?}` |
| Language id | `cpp-gpp-64` (`lang/cpp-gpp-64.yaml`) |
| Frontend routes | `/`, `/problems`, `/problems/:slug`, `/login`, `/register`, `/contests`, `/submissions/:id` |
| Token storage | `localStorage["access_token"]`, `localStorage["refresh_token"]` |
| No existing e2e | `web/e2e` does not exist; `web/package.json` has no `test:e2e` |

**Tech Stack:** Docker Compose v2, Go 1.x (`cmd/seed` + `internal/seed`), Playwright 1.6x
(run from host `web/` against `http://localhost:8081`), MailHog.

**Spec:** `docs/superpowers/specs/2026-06-13-full-scale-completion-design.md` — §5 (sim
architecture), §6 (golden path), §8 (done criteria), §9 (Wave 0 exit).

## Global Constraints

- **Do not touch `docker-compose.yml`.** Every sim-specific knob lives in the new overlay
  `docker-compose.sim.yml`.
- **No new `--mode` in `cmd/aioj/main.go`, no new Dockerfile, no supervisor container.** The
  seeder is `cmd/seed` invoked by `docker compose --profile sim run --rm seeder`.
- Backend changes allowed in this wave, complete list: **none.** Zero `internal/` edits are
  required — register/login/problem/testcase/submit already work. If a task seems to need an
  `internal/` change, STOP: the misunderstanding is in the task, not the code.
- No `make` targets are invented that hide docker commands; a thin `Makefile` addition is fine
  but must be one-line wrappers only.
- Playwright runs on the **host** (needs a browser, not a container). It hits
  `http://localhost:8081`. MailHog API is read at `http://localhost:8025/api/v2/messages`.

---

### Task 1: `docker-compose.sim.yml` overlay

**Files:** Create `docker-compose.sim.yml`

- [ ] **Step 1: Write the overlay**

```yaml
# Simulation overlay for the Full-Scale Completion program.
# Boots on top of the base file; never modifies it:
#   docker compose -f docker-compose.yml -f docker-compose.sim.yml --profile sim up -d --build
services:
  frontend:
    # Rebind to 8081 so the sim never fights local dev servers / Caddy on :80.
    ports: !reset []
    ports:
      - "8081:80"

  # Two deciders on one Redis queue — the Wave 1 lossless/HA proof starts here.
  judge-worker:
    deploy:
      replicas: 2

  # Wave 1+ reads verification mail from here; Wave 0 brings it up so the
  # service is present and healthy in every sim boot.
  mailhog:
    image: mailhog/mailhog:latest
    ports:
      - "8025:8025"   # web UI + JSON API  (localhost:8025/api/v2/messages)
      - "1025:1025"   # SMTP
    restart: unless-stopped

  # One-shot API seeder (cmd/seed). Runs after backend is healthy, exits 0.
  seeder:
    build:
      context: .
      dockerfile: Dockerfile
    command: ["/app/seed"]
    environment:
      DB_HOST: postgres
      DB_PORT: "5432"
      DB_NAME: aioj
      DB_USER: aioj
      DB_PASSWORD: ${DB_PASSWORD:-aioj_secret}
      SEED_ADMIN_USERNAME: ${SEED_ADMIN_USERNAME:-ai}
      SEED_ADMIN_PASSWORD: ${SEED_ADMIN_PASSWORD:-Aioj-Sim-Admin-2026!}

      SEED_PROBLEM_SLUG: ${SEED_PROBLEM_SLUG:-hello}
      BACKEND_URL: http://backend:8080
    depends_on:
      backend:
        condition: service_started
    profiles: ["sim"]
```

- [ ] **Step 2: Validate the syntax without booting**

```bash
docker compose -f docker-compose.yml -f docker-compose.sim.yml config >/dev/null && echo OK
```

Expected: `OK`. If it errors, the most likely cause is a stale `docker-compose.override.yml`
being auto-merged; remember Compose merges `docker-compose.override.yml` **automatically** when
it sits next to the base file — that is fine here (it only resets postgres ports).

- [ ] **Step 3: Commit**

```bash
git add docker-compose.sim.yml
git commit -m "sim: compose overlay (mailhog, 2 workers, frontend :8081, seeder)"
```

---

### Task 2: `internal/seed` + `cmd/seed` — API-driven seeder

**Files:** Create `internal/seed/seed.go`, `internal/seed/seed_test.go`, `cmd/seed/main.go`

**Interfaces:**

```go
package seed

// Config is every knob the sim needs; all optional, all have safe defaults.
type Config struct {
	BackendURL  string // http://backend:8080
	AdminUser   string // "ai"
	AdminPass   string // "aiseedpass"
	AdminEmail  string // derived
	ProblemSlug string // "hello"
}

// Result records what the seeder actually did, so a test can assert idempotence.
type Result struct {
	AdminCreated   bool
	AdminPromoted  bool   // true only if role was flipped via the admin API
	ProblemCreated bool
	TestcaseCount  int
	Errors         []error
}

// Seed is idempotent: a second run on a populated DB is a no-op that returns
// AdminCreated=false, ProblemCreated=false.
func Seed(ctx context.Context, cfg Config) (Result, error)
```

- [ ] **Step 1: Write the failing unit test (no network, no DB)**

`internal/seed/seed_test.go` — test the pure pieces: default-filling, the admin-promotion
decision, and zip construction. The HTTP layer is exercised by the real sim boot in Task 5.

```go
func TestSeed_defaultsAreNeverEmpty(t *testing.T) {
	cfg := fillDefaults(Config{})
	if cfg.BackendURL == "" || cfg.AdminUser == "" || cfg.AdminPass == "" || cfg.ProblemSlug == "" {
		t.Fatalf("defaults must be non-empty: %+v", cfg)
	}
}

func TestSeed_buildHelloTestcases(t *testing.T) {
	b, err := buildHelloTestcases()
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("want 2 entries (1.in,1.out), got %d", len(zr.File))
	}
}
```

- [ ] **Step 2: Run and watch it fail to compile**

```bash
go test ./internal/seed/
```

Expected: FAIL — `package internal/seed does not exist` / `undefined: fillDefaults`.

- [ ] **Step 3: Implement `internal/seed/seed.go`**

Behavior, in order:

1. `fillDefaults` — never returns an empty field.
2. `POST /api/auth/register` with `{adminUser, adminUser+"@aioj.test", adminPass}`.
   - 201 → remember `access_token` and `user.id`.
   - 409 (`username taken`/`email taken`) → `POST /api/auth/login` instead, then fetch
     `GET /api/users/{username}` for the id. **Do not** treat 409 as an error; it is the
     idempotent path.
3. Promote to admin. There is no bootstrap-admin endpoint, so the seeder promotes **directly in
   Postgres**: `UPDATE users SET role='admin' WHERE username=$1`. This is the one DB write in
   the whole harness and it is deliberately confined to the seeder (the product never needs it;
   every later promotion goes through `PUT /api/admin/users/{id}/role`).
   - After the UPDATE, re-login so the JWT carries `role: admin`.
4. Create the problem via `POST /api/problems` (auth, admin JWT):

```json
{
  "slug": "hello",
  "title": "Hello, AIOJ!",
  "description": "Print `Hello, AIOJ!`.",
  "input_format": "No input.",
  "output_format": "The string `Hello, AIOJ!`.",
  "time_limit": 1000, "memory_limit": 262144, "difficulty": "easy",
  "sample_cases": [{"input": "", "output": "Hello, AIOJ!"}],
  "checker_type": "exact",
  "visible": true
}
```

   - 201 → `ProblemCreated=true`. 409/already-exists → fetch by `GET /api/problems/{slug}`,
     set `ProblemCreated=false`, keep going.
5. Upload testcases: `POST /api/problems/{slug}/testcases`, multipart field `file`, zip payload
   from `buildHelloTestcases()` containing `1.in` (empty) and `1.out` (`Hello, AIOJ!`).
   - 400 if the zip is malformed; ignore "already has testcases" style responses and just count.
6. Return `Result`. Any *unexpected* HTTP status → append to `Result.Errors` and return the
   error; the seeder must fail loudly rather than leave a half-seeded sim.

`buildHelloTestcases()` is a pure function: builds an in-memory zip with `archive/zip` +
`bytes.Buffer`. That is why Task 2's test needs no network.

- [ ] **Step 4: Run the unit tests green**

```bash
go test ./internal/seed/ -v
```

Expected: PASS for both tests.

- [ ] **Step 5: `cmd/seed/main.go`**

A thin shell — no logic. Reads env (see Task 1 env block), calls `seed.Seed`, prints a one-line
JSON summary to stdout, exits non-zero on error so `docker compose run` surfaces the failure.

```go
func main() {
	cfg := seed.Config{
		BackendURL:  envOr("BACKEND_URL", "http://backend:8080"),
		AdminUser:   envOr("SEED_ADMIN_USERNAME", "ai"),
		AdminPass:   envOr("SEED_ADMIN_PASSWORD", "aiseedpass"),
		ProblemSlug: envOr("SEED_PROBLEM_SLUG", "hello"),
	}
	res, err := seed.Seed(context.Background(), cfg)
	out, _ := json.Marshal(res)
	fmt.Printf(`{"seed":%s}`+"\n", out)
	if err != nil { os.Exit(1) }
}
```

- [ ] **Step 6: Local compile check (no DB needed)**

```bash
go build ./cmd/seed && go vet ./internal/seed/ ./cmd/seed/ && echo BUILT
```

Expected: `BUILT`.

- [ ] **Step 7: Commit**

```bash
git add internal/seed/ cmd/seed/
git commit -m "sim: API+PG seeder for admin/problem/testcases (idempotent)"
```

---

### Task 3: Playwright install + config

**Files:** `web/package.json` (1 script + 1 devDep), `web/playwright.config.ts`, `web/e2e/lib/api.ts`

- [ ] **Step 1: Install Playwright**

```bash
cd web && npm install -D @playwright/test && npx playwright install chromium
```

Expected: `@playwright/test` in devDependencies; chromium downloaded. If `npx playwright install`
fails offline, `brew install --cask` is not needed — Playwright can use a system Chromium; record
the workaround in the pitfall list and continue (Task 4 will surface it as a hard failure).

- [ ] **Step 2: Add the script**

In `web/package.json` `scripts`:

```json
"test:e2e": "playwright test"
```

- [ ] **Step 3: `web/playwright.config.ts`**

```ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  expect: { timeout: 15_000 },
  fullyParallel: false,        // one browser, one OJ — sim state is shared
  retries: 1,
  reporter: [['html', { outputFolder: 'e2e/artifacts/report' }], ['list']],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:8081',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
});
```

- [ ] **Step 4: `web/e2e/lib/api.ts` — typed helper for the real API**

Every spec uses this instead of hand-rolling `fetch`, so the golden path asserts against the
*real* contract (not the DOM's interpretation of it):

```ts
const BASE = process.env.E2E_BASE_URL ?? 'http://localhost:8081';

export async function register(u: string, e: mailhog-ish string, p: string) { /* POST /api/auth/register */ }
export async function login(u: string, p: string) { /* POST /api/auth/login → tokens */ }
export async function health() { /* GET /api/health → {status:'ok'} */ }
```

- [ ] **Step 5: Verify the harness is wired (zero tests is fine)**

```bash
cd web && npx playwright test --list
```

Expected: exit 0, no tests listed, no config errors.

- [ ] **Step 6: Commit**

```bash
git add web/package.json web/package-lock.json web/playwright.config.ts web/e2e/lib/api.ts
git commit -m "sim: playwright harness wired to :8081"
```

---

### Task 4: Health + register + login specs (the first real browser tests)

**Files:** `web/e2e/health.spec.ts`, `web/e2e/auth.spec.ts`

- [ ] **Step 1: Boot the sim (this is the first full boot; expect 3–6 min cold)**

```bash
docker compose -f docker-compose.yml -f docker-compose.sim.yml --profile sim up -d --build
```

Then wait for the stack:

```bash
for i in $(seq 1 60); do
  curl -sf http://localhost:8081/api/health >/dev/null && echo "HEALTHY" && break
  sleep 2
done
```

Note: `/api/health` is served by the **backend**; nginx proxies `/api/*` to it (verify in
`web/nginx.conf` — if it does not, point the health test at `:8080` directly and record the
proxy gap as a Wave-0 finding).

- [ ] **Step 2: Run the seeder**

```bash
docker compose -f docker-compose.yml -f docker-compose.sim.yml --profile sim run --rm seeder
```

Expected: JSON line with `AdminCreated:true, ProblemCreated:true` on a fresh volume; on a
re-run, both `false` and exit 0 (idempotence).

- [ ] `web/e2e/health.spec.ts`

```ts
import { test, expect } from '@playwright/test';

test('backend health is ok through nginx', async ({ request }) => {
  const r = await request.get('/api/health');
  expect(r.status()).toBe(200);
  expect(await r.json()).toMatchObject({ status: 'ok' });
});
```

- [ ] `web/e2e/auth.spec.ts` — UI register → stored tokens → protected route

```ts
import { test, expect } from '@playwright/test';

const uniq = () => `sim${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;

test('register via UI stores a real session', async ({ page }) => {
  const user = uniq();
  await page.goto('/register');
  await page.getByPlaceholder(/username/i).fill(user);
  await page.getByPlaceholder(/email/i).fill(`${user}@aioj.test`);
  await page.getByPlaceholder(/^password/i).fill('strongpass1');
  await page.getByRole('button', { name: /register/i }).click();

  // SPA sets tokens on success; a hard navigation is NOT required.
  await expect.poll(async () => page.evaluate(() => localStorage.getItem('access_token')))
    .toBeTruthy();
  await expect.poll(async () => page.evaluate(() => localStorage.getItem('refresh_token')))
    .toBeTruthy();
});

test('login via UI as the seeded admin', async ({ page }) => {
  await page.goto('/login');
  await page.getByPlaceholder(/username/i).fill(process.env.SEED_ADMIN_USERNAME ?? 'ai');
  await page.getByPlaceholder(/^password/i).fill(process.env.SEED_ADMIN_PASSWORD ?? 'aiseedpass');
  await page.getByRole('button', { name: /login/i }).click();
  // Admin-only route proves the JWT role is wired end to end:
  await page.goto('/admin');
  await expect(page.getByRole('heading', { name: /admin/i }).or(page.getByText(/dashboard/i)))
    .toBeVisible();
});
```

- [ ] **Step 3: Run them green**

```bash
cd web && npx playwright test e2e/health.spec.ts e2e/auth.spec.ts
```

Expected: 3 passed. If `/register` placeholders do not match, fall back to positional `input`
locators and **record the actual DOM shape** in this plan's pitfalls (do not guess again).

- [ ] **Step 4: Commit**

```bash
git add web/e2e/health.spec.ts web/e2e/auth.spec.ts
git promote && git commit -m "sim: health + register/login e2e green on :8081"
```

---

### Task 5: The milestone — judge Hello World to AC

**Files:** `web/e2e/judge.spec.ts`, `web/e2e/lib/api.ts` (add submit + poll helpers)

- [ ] **Step 1: Add API helpers**

```ts
export async function submit(token: string, problemId: string, language = 'cpp-gpp-64', code: string) {
  // POST /api/submissions  {problem_id, language, source_code}  → 201 {id,status}
}
export async function getSubmission(token: string, id: string) {
  // GET /api/submissions/{id}  → {id,status,verdict,time_used,memory_used}
}
```

The canonical AC source:

```cpp
#include <iostream>
int main(){ std::cout << "Hello, AIOJ!\n"; }
```

- [ ] `web/e2e/judge.spec.ts`

```ts
import { test, expect } from '@playwright/test';
import { login, submit, getSubmission } from './lib/api';

test('submit cpp → verdict AC, persisted', async ({ request, page }) => {
  const { access_token } = await login(request, 'ai', process.env.SEED_ADMIN_PASSWORD ?? 'aiseedpass');
  const problem = await getBySlug(request, 'hello');          // GET /api/problems/hello
  const sub = await submit(request, access_token, problem.id);
  expect(sub.status).toBe('pending');

  // Poll the real API until a terminal verdict (worker is compiled+run inside go-judge).
  let verdict = sub.status;
  for (let i = 0; i < 60 && !['ac','wa','tle','mle','re','ce'].includes(verdict); i++) {
    await page.waitForTimeout(1000);
    verdict = (await getSubmission(request, access_token, sub.id)).status;
  }
  expect(verdict).toBe('ac');

  // The same verdict must survive a reload — no phantom AC.
  await page.goto(`/submissions/${sub.id}`);
  await expect(page.getByText(/accepted/i)).toBeVisible({ timeout: 15_000 });
});
```

- [ ] **Step 2: Run the milestone**

```bash
cd web && npx playwright test e2e/judge.spec.ts
```

Expected: PASS. While it runs, sanity-check the cluster shape:

```bash
docker compose -f docker-compose.yml -f docker-compose.sim.yml --profile sim ps
# judge-worker must show 2/2 replicas — the Wave 1 HA proof starts from this shape.
```

- [ ] **Step 3: Full suite green**

```bash
cd web && npx playwright test
```

Expected: **4 files, all green** (health, auth×2, judge).

- [ ] **Step 4: Commit**

```bash
git add web/e2e/judge.spec.ts web/e2e/lib/api.ts
git commit -m "sim: hello-world judged to AC on the two-worker sim stack"
```

---

### Task 6: `make` wrappers + Wave-1 handoff

**Files:** `Makefile` (append), `docs/superpowers/specs/2026-06-13-full-scale-completion-design.md` (§9 note)

- [ ] **Step 1: Append thin wrappers** (each is exactly the command from the tasks above)

```make
SIM = docker compose -f docker-compose.yml -f docker-compose.sim.yml --profile sim

.PHONY: sim-up sim-down sim-seed sim-logs e2e sim-reset

sim-up:
	$(SIM) up -d --build

sim-down:
	$(SIM) down

sim-reset:   # clean boot from scratch — the §8 "wave done" gate
	$(SIM) down -v && $(SIM) up -d --build

sim-seed:
	$(SIM) run --rm seeder

sim-logs:
	$(SIM) logs -f backend judge-worker

e2e:
	cd web && npx playwright test
```

- [ ] **Step 2: Backfill spec §9** with one sentence: dev/prod-image rehearsal is the same test
  list; only the frontend image differs (`web/Dockerfile` prod build vs. a dev-server override).
  Milestone 2 = same suite green after `make sim-reset` from a cold engine.

- [ ] **Step 3: Clean-boot gate (the real exit criterion)**

```bash
make sim-reset && make sim-seed && make e2e
```

Expected: `make e2e` exits 0. This is the Wave 0 exit — **do not claim the wave is done before
this exact sequence passes from a cold Docker engine.**

- [ ] **Step 4: Commit**

```bash
git add Makefile docs/superpowers/specs/2026-06-13-full-scale-completion-design.md
git commit -m "sim: make wrappers + wave-0 exit gate"
```

---

## Pitfalls

1. **`docker-compose.override.yml` auto-merges.** Compose loads it automatically because it sits
   beside the base file. It only resets postgres ports here, which is harmless — but if a port
   conflict appears, `-f` order is: base, sim, then the auto override.
2. **Two `ports:` keys under `frontend`.** The `!reset []` line is required to clear the base
   `80:80` before re-binding to `8081:80`; without it Compose errors on duplicate keys at the
   same level. (YAML forbids true duplicate keys; `!reset` is the supported escape hatch.)
3. **First boot is slow.** ~3–6 min cold (Go build + npm build + postgres init + judge image).
   The health poll loop in Task 4 Step 1 is the supported wait, not a fixed `sleep`.
4. **There is no bootstrap-admin API.** `PUT /api/admin/users/{id}/role` needs an existing admin.
   The seeder therefore promotes the first admin directly in Postgres — one DB write, confined
   to the seeder, never the product. Every later promotion uses the real API.
5. **`/api/health` is a backend route.** Verify `web/nginx.conf` proxies `/api/*`; if not, the
   health spec must target `:8080` and that proxy gap becomes a Wave-0 finding, not a guess.
6. **judge-worker `replicas: 2` + `restart: unless-stopped`.** Both replicas share one Redis
   queue; a duplicate verdict would be a real Wave 1 bug, not a sim artifact. Watch for it in
   `make sim-logs` while the milestone test runs.
7. **Rate limiter is `RemoteAddr`-keyed.** With 2 workers behind one backend the client IP is
   constant, so a fast e2e loop can trip the 100 rps / 200 burst limit. The golden path stays
   far under it; if a wave later adds load tests, they must stagger.
8. **Vite prod build ≠ dev server.** The sim runs the *prod* nginx image (no HMR, no port
   spray). Any "just run `npm run dev`" instinct is wrong for this harness.
