# Design audit: SOLID and code hygiene

A read of the ~55k lines of Go (`internal/`, `cmd/`) and ~44k lines of TypeScript
(`web/src`) against single-responsibility, interface-segregation and
open/closed expectations. Recorded so the debt is visible and the ratchet in
`.golangci.yml` has something to point at.

Two things up front. First, this codebase is unusually disciplined: it has a real
composition root, consumer-side interfaces, one textbook open/closed registry,
and comments that document past bugs with the reasoning that fixed them. Second,
every finding below is **pre-existing debt**. The build is green, and the linter
is a `new-from-merge-base: main` ratchet, so nothing here fails CI — it is the
floor the next change starts from.

Severity reflects cost to change and likelihood of causing a bug, not taste.

---

## Findings

### 1. `Deps` is a service locator and `NewRouter` was a second program — **High** (partly fixed) (router split fixed)

`internal/api/deps.go:14`, `internal/api/router.go`

`Deps` has **77 fields**. `NewRouter` was **893 lines** — longer than any file in
`internal/store/` — doing four jobs: unpacking dependencies (46 `xH := d.X`
aliases), building the feature-gate closure, assembling middleware, and
registering all 408 routes.

**The route registration is split.** `NewRouter` is now 58 lines: it assembles
the base chain and calls 16 `mount<Domain>` methods, each in its own
`router_mount_*.go` file. The shared value they receive is `routerBuilder`
(`router_builder.go`), which embeds `*Deps` so there is still exactly one place
listing what the router may depend on.

The split is verified rather than asserted. `router_table_test.go` walks the real
chi router and pins all 294 mounted routes **with their middleware chains**, in
`testdata/route_table.golden`. Middleware is named semantically
(`AuthMiddleware`, `RequireRole`, …) because the runtime name of a closure
changes under refactoring while its behaviour does not. Two focused assertions
sit on top: every `/api/admin/*` route must carry both `AuthMiddleware` and
`RequireRole`, and eight critical endpoints must stay mounted.

**Still open:** `Deps` is still a 77-field service locator, and optionality is
still encoded as `if d.X != nil` across **32** sites rather than in the type.
Grouping `Deps` into required and optional sub-structs remains the fix.

That refactor is now safe to attempt because both halves of the contract are
pinned: `TestMissingDepsDoNotPanic` requires a partially-wired router to answer
404 rather than panic, and `api.ValidateWiring` refuses to boot with a required
handler unwired.

The second one is a fix, not just a guard. A nil handler is a valid pointer, so
a handler added to a mount file but forgotten in `main` compiled, started, and
then panicked on the nil receiver for every request to that route — recovered
into an empty 500 that named nothing. Verified directly:
`(*handler.GymHandler)(nil).List` is a nil-pointer dereference. `main` now aborts
at boot with every gap named, and `TestOptionalListMatchesMountGuards` keeps the
hand-maintained optional list honest against the actual guards in the mount files.

**The remaining half is done.** `Deps` now has the 51 required fields at the top
level and the 27 optional ones in `Deps.Optional`, so "may be unset" is expressed
by where a field lives. `ValidateWiring` checks only the top level, and
`TestOptionalFieldsLiveInOptional` parses the mount files to assert every
nil-guarded field is declared on `OptionalDeps` — the structural version of the
check that used to keep a hand-written map honest. The map is gone.

### 2. Handlers depend on the Postgres implementation, not the interfaces — **High**

24 handler files import `internal/store/postgres`, and the concrete type appears
in constructor *signatures*:

```go
func NewContestHandler(s *postgres.ContestStore, ...)        // contest.go:155
func WithGrants(gs *postgres.ContestGrantStore) ...          // 7 handlers
```

The `store.*Store` interfaces already exist. This blocks ever testing handlers
without Postgres and inverts the dependency direction the rest of the design
assumes.

**Fixed** — see "Repairs made" below.

### 3. Store interfaces far wider than any consumer — **High** (closed)

`internal/store/interfaces.go`

Traced per consumer:

| Consumer | Methods actually used |
|---|---|
| `judge/reclaimer.go` | 2 (`ListPendingWithContext`, `RequeueStale`) |
| `hack/service.go` | 1 (`GetByID`) |
| `plagiarism/service.go` | 1 (`ListByProblem`) |
| `handler/users.go` | 1 (`ListPublicByUser`) |
| `handler/aggregator.go` | 2 |
| `handler/contest_adjustment.go` | 2 |
| `handler/stats.go` | 3 (`Get*Stats`) |
| `judge/worker.go` | 3 |

Every consumer is forced to depend on 22–27 methods it never calls, which is
what forces 30-method test stubs. Same shape in `ContestStore` (25),
`UserStore`/`ProblemStore`/`TrainingPlanStore` (21), `GroupStore` (19),
`AIGenerationStore` (19).

*Direction:* carve along the consumer axes above. `ContestGrantStore` is the
in-house proof it works — it was split out for exactly this reason.

**Closed.** `SubmissionStore`, `ContestStore`, `UserStore` and `ProblemStore` are
now role sets with the wide name kept as a union for the one consumer that spans
it. See "Repairs made".

**Fixed for `SubmissionStore`** — see "Repairs made" below. The same carve is
still open for `ContestStore` (25), `UserStore`/`ProblemStore`/
`TrainingPlanStore` (21), `GroupStore` (19) and `AIGenerationStore` (19).

### 4. The remote-judge open/closed design was bypassed where it existed — **High** (fixed, plus a real bug)

`internal/vjudge/bot.go` defined a proper `Bot` interface with `RegisterBot`,
and `service.go` then special-cased one platform inline in three places:

- `service.go:186` built the remote submission URL behind `if req.RemoteOJ == "codeforces"`, so **every other platform stored an empty `remote_url`** and its submission page offered no link back to the source judge.
- `service.go:349` `ForcePoll` hand-rolled a Codeforces `user.status` fetch instead of calling `bot.Poll`.
- `service.go:472` `if platform == "codeforces" { pollCF } else { pollGeneric }`.

Plus `handler/admin_bot_accounts.go:228,268` (a platform ladder for credential
validation) and `cmd/aioj/main.go:304` (`switch platform` with five hardcoded
constructors, so a sixth platform meant editing the composition root).

**Fixed.** All three service branches are gone; zero platform-name literals
remain in `service.go`, `main.go` or `admin_bot_accounts.go`. The seams:

| What the service asks | Interface | Why it is a capability, not a branch |
|---|---|---|
| "what is this submission's URL?" | `Bot.SubmissionURL` | URL shape is platform knowledge; nothing else has it |
| "can you report many verdicts at once?" | `BatchPoller` | only Codeforces can, so it is optional — 4 of 5 platforms honestly cannot |
| "can you validate cookies?" | `CookieValidator` | only Codeforces has a probe; the rest must not be reported as validated |

Registration mirrors `contest/format/registry.go`: `RegisterBotFactory` +
`init()` per platform, with `RegisteredPlatforms()` driving `main.go`. Adding a
platform is now one new file in `internal/vjudge` plus a line in
`model.RemotePlatforms` — which its own comment already documented as the only
edit.

**A real bug fell out of this.** The verdict mapping lived in four copies (the
force-poll path, the generic path, and one normalizer each in QOJ and Toph), and
**none of them knew about `MLE`** — although three bots emit it. A remote
submission that exceeded its memory limit was recorded as **Wrong Answer** on
every platform, silently corrupting standings. The two platform normalizers also
returned `""` for an unlisted verdict, which is indistinguishable from "still
judging", so such a submission was polled until it timed out.

There is now one mapper, `vjudge.Verdict` / `normalizeVerdict` / `Status()`, and
one place that writes a remote result back (`Service.finishRemote`). `OLE` and
`PE` map to WA deliberately — the verdict enum has no slot for either, and the
raw platform verdict is still logged.

Guarded by `verdict_test.go`: the spellings each platform actually sends, the
`MLE` regression, a check that every verdict the bots emit survives the round
trip, a check that every enum slot is reachable, the extracted
`mapCFSubmissionBatch` (nil-tolerant, matches only requested ids), and a
registry check that `RegisteredPlatforms()` and `model.RemotePlatforms` agree in
both directions. Each was confirmed to fail when the fix is reverted.

### 5. Hand-rolled `fetch` calls in the API client skip auth — **High** (fixed)

`web/src/lib/api.ts`

`request()` refreshes the token on 401 and attaches `X-CSRF-Token`. **Eight**
endpoints bypassed it with raw `fetch` and `if (token) headers['Authorization'] = …`,
so none of them handled 401/refresh.

Because the CSRF middleware only skips its check when an `Authorization` header
is *present* (`internal/api/middleware/csrf.go:36`), an expired access token
produced **403 "csrf cookie missing"** — an unrecoverable error — instead of a
401 the client could retry. This was the one finding with a user-visible symptom.

**Fixed** — see below.

### 6. `main()` is a second program — **Medium**

`cmd/aioj/main.go:53`, **919 lines**

Parses flags, connects the DB, runs migrations, wires Redis, mail, queue, the
judge worker pool, the vjudge pollers, the HTTP server, two background loops and
graceful shutdown, then dispatches four run modes via scattered
`if *mode == "…"` checks at lines 186, 509, 528.

*Direction:* one `run<Mode>(cfg, deps) error` per mode, with `main` doing flag parsing and dispatch only.

### 7. Business rules live in the HTTP layer — **Medium**

`internal/api/handler/contest.go:895`

`Scoreboard` is **320 lines** performing cache-key construction, standings
computation, freeze logic, manual-correction application, ranking,
re-pagination and serialization. `Create` is 159 lines and `Update` 208.

*Direction:* the scoring itself already exists behind `format.ContestFormat`; move the assembly into a service the handler calls.

### 8. Report/algorithm logic in the Postgres layer — **Medium**

`internal/store/postgres/recommendations.go:67`

`GetRecommendations` is **159 lines** of SQL plus algorithm — difficulty
banding, weak-tag identification, ordering policy — in the persistence package.
It is a query object, not a repository method.

### 9. Errorless store contracts — **Low** (partly fixed)

Original finding covered three contracts. Re-reading them separated one real
defect from two deliberate choices.

`GetUsername(ctx, id) string` **was** a defect: `postgres/contests.go:545` ran
`QueryRow(...).Scan(...)` and discarded the error, returning `""` for both "no
such user" and "database down", and it was called once per participant inside
the scoreboard and rating loops — 500 queries for the names on a 500-entry
board.

**Fixed**: added `GetUsernames(ctx, ids) (map[string]string, error)`, one query
for the page and one that reports its failure. The scoreboard falls back to
blank on a failed lookup (a name must not 500 a standings page); the rating
recalculation aborts, because it feeds an irreversible write. `GetUsername`
survives as a documented best-effort single-row helper for the one caller that
has a bare id.

`HasAccess(...) bool` (×2) is **not** a defect and should stay as it is. It is
called from 52 sites as a permission check, and returning `false` on a database
error is the fail-closed answer. Converting it to `(bool, error)` would move the
error decision into 52 callers, where the cheap mistake is
`if err != nil { return true }`. The comment there now says so explicitly.

### 10. Inconsistent response shapes — **Medium**

**1191** `http.Error` sites (plain-text body) against **334** `respondJSON`
(JSON body), across `internal/api/handler/`. There is no shared error helper;
`respondJSON` is defined incidentally in `auth.go:561` and reached by package
scope. Some admin endpoints signal failure with **HTTP 200 + `{"ok": false}`**
(`admin_ai_models.go:265,277`).

### 11. Test doubles compile into the production binary — **Medium**

`internal/api/handler/test_stubs_email.go` — 30 stub types, 4.4 KB, no build
tag, referenced by nothing outside tests, and confirmed present in `go list`
output for the package.

*Direction:* rename to `_test.go` or add `//go:build test`.

### 12. Config read three different ways — **Medium**

**79** `os.Getenv` calls. `config.Load` (`config/config.go:253`, itself 198
lines) does it properly; `internal/plagiarism/`, `internal/seed/` and
`cmd/aioj/main.go` (e.g. `FEED_TOKEN`) read the environment directly, bypassing
validation and defaults.

### 13. God page components — **Medium** (partly fixed; the headline claim corrected)

`web/src/pages/ContestManage.tsx` was **2062 lines**. The first pass called it a
god component on the strength of "50 `useState`", which overstated the case: the
state is spread across twelve components, and no single function held most of it.

Measuring the functions directly found the real outlier: **`SettingsTab` was 768
lines** — 37% of the file — a 25-field form with its load, save, publish and eight
pieces of state, and ~700 lines of JSX in one return. That is the item worth
splitting, and it has been.

The rest of the file is thirteen tab components of 50–120 lines each, which is a
long page rather than an incoherent one.

**Fixed:** `SettingsTab`'s two self-contained blocks are now
`components/ContestManage/AccessTable.tsx` (the per-surface audience table) and
`FormatSettings.tsx` (the selected format's own numbers, registry-driven),
following the existing `components/SetterWorkspace/` precedent. State stays in the
parent because both are one field of a form saved as a single PUT. 2062 → 1893
lines, `SettingsTab` 768 → ~710, ten new tests.

The other large pages — `ContestDetail.tsx` (1204), `Profile.tsx` (1226),
`ContestProblem.tsx` (1017), `ProblemDetail.tsx` (853) — are untouched. Also still
true: 18 `await api.*` calls and 41 `setLoading`/`setError` sites in ContestManage
with no shared fetch hook, which is the duplication half of this finding.

`ContestDetail.tsx` (1204), `Profile.tsx` (1226), `ContestProblem.tsx` (1017),
`ProblemDetail.tsx` (853) have the same shape.

### 14. `With*` mutators as a second constructor — **Low** (re-examined: not a defect)

**20** `With*` methods attach an optional collaborator after construction.

On re-examination this is a **false positive**, recorded here rather than
"fixed". Three things make it sound:

- **The constructor always leaves a usable handler.** `NewContestHandler` builds
  a fail-closed access policy with a nil grant store, so a handler built without
  `WithGrants` refuses password rounds rather than opening them. The optional
  dependency has a safe zero value; nothing is half-initialised.
- **The fluent form is idiomatic Go.** Returning the receiver for chaining is the
  standard builder shape, and every call site in `main.go` uses it that way.
- **Coverage is complete and checkable.** Seven handlers embed the grant-aware
  access policy and `main.go` wires exactly seven `WithGrants` calls — no handler
  is left on the fail-closed default by accident.

The one genuine cost is that a forgotten `With*` is invisible to the compiler. It
is not worth a redesign: the failure mode is a refused request, not a security
hole, and the count is small enough to check by grep.

The five `With*` calls that rebuild the access policy (`rebuildWithGrants`) do
re-derive it, but only once per handler at startup, which is not a hot path.

---

## Checked and found sound

Recorded so these are not "fixed" later:

- `web/src/App.tsx` (497 lines, 68 routes) is a clean lazy-loaded route table.
- `lib/api.ts`'s `api` object is ~420 endpoint definitions — a catalogue, not
  mixed concerns.
- Frontend type discipline is good: **1** `any`, 0
  `dangerouslySetInnerHTML`, 114 `as` casts (mostly legitimate DOM/union
  narrowing).
- `hooks/useContestAnnouncements.ts`, `hooks/useClarifications.ts` are properly
  extracted hooks.
- `internal/contest/format/` is a correct open/closed plugin registry.
- `HasAccess(...) bool` returning a bare bool is **deliberate fail-closed**
  design (see finding 9), not a defect.
- `TestConnection`'s HTTP 200 with `{"ok": false}` is a deliberate diagnostic
  contract the admin panel branches on (see finding 10).
- Handler *bodies* are clean: exactly **one** raw SQL statement in the whole
  handler package (`user_cascade_deleter.go:26`).
- `contest_access.go` is a genuine single-policy seam with a documented
  security rationale.

---

## Repairs made

Two findings were fixed in this pass; the rest are recorded above.

### Finding 5 — API client auth and CSRF

`web/src/lib/api.ts` now extracts `fetchAuthed` (token + CSRF + one 401
refresh-retry), `throwForStatus` and `requestBlob` out of `request()`. All eight
bypassing endpoints — `uploadTestcases`, `downloadTestcase`, `importProblem`,
`importCodeforces`, `importCSES`, `importAtCoder`, `importToph`, `importQOJ` —
now go through that path.

Covered by `web/src/lib/api.authRetry.test.ts`, which fails against the previous
implementation on its two refresh-retry cases.

### Finding 2 — concrete stores in handler signatures

24 handler files plus `internal/hack` now take `store.*` interfaces instead of
`*postgres.*`. Three things surfaced while doing it, each worth noting:

1. **The interfaces were incomplete.** Six methods were called by handlers but
   declared nowhere: `GetScoreboardRows`, `GetContestStats`, `RegisterTeam`,
   `ListTeamRegistrations`, `GetByIDForViewer`, `ListUsersByRating`. They are now
   declared.
2. **Row types crossed the store boundary.** `ScoreboardRow`,
   `ContestProblemStats` and `ContestStats` were declared in `postgres`, which
   imports `store` — so `store` could not name them. Moved to `internal/model`
   with aliases left behind in `postgres`.
3. **Narrow seams replaced wide ones.** `RankingsHandler` used exactly one
   `UserStore` method; `hack.Service` and the gym/group/editorial/virtual
   handlers used one `ContestStore` method each. Rather than widen shared
   interfaces (which would worsen finding 3), each got a one-method
   consumer-side interface, following the existing `feedLister` /
   `contestLookup` precedent.

Verified: `go build ./...` and `go vet ./internal/... ./cmd/...` clean; the
failing-test set is byte-identical to the pre-change baseline (12 pre-existing
failures, all `column "access_config" does not exist` against a stale test
schema).

### Finding 1 — `NewRouter`

Route registration moved out of the 893-line function into 16 `mount<Domain>`
methods, one file each:

| File | Lines | Domain |
|---|---|---|
| `router.go` | 58 | `NewRouter`: base chain + mount order |
| `router_builder.go` | 57 | `routerBuilder`, `gate`, `base` |
| `router_mount_public.go` | 112 | metrics, health, status, legal, meta, sitemap, feeds, search |
| `router_mount_admin.go` | 146 | `/api/admin` (auth + admin role) |
| `router_mount_problems.go` | 99 | archive, i18n, testcases, import |
| `router_mount_orgs_training.go` | 83 | organizations, classes, training, plagiarism |
| `router_mount_social.go` | 82 | notifications, groups, teams, blog |
| `router_mount_contest_extras.go` | 80 | adjustments, clarifications, notices, register, onsite |
| `router_mount_contests.go` | 67 | contests, scoreboard, unlock, booklet |
| `router_mount_auth.go` | 62 | csrf, login, register, 2FA, OAuth |
| `router_mount_virtual_gym_hacks.go` | 48 | virtual, gym, hacks, stats |
| `router_mount_ai.go` | 47 | AI jobs |
| `router_mount_content.go` | 46 | editorials, keys, webhooks, recommendations |
| `router_mount_users.go` | 37 | public profile, social graph, profile edit |
| `router_mount_misc_top_level.go` | 31 | remote-languages, setter-apply, ws, rating |
| `router_mount_submissions.go` | 31 | submissions, run, upsolve |
| `router_mount_public_apiv1.go` | 30 | `/api/v1` (API-key auth) |

Every registration body moved **verbatim** — comments included — with only the
alias declarations rewritten (`authH := d.Auth` becomes `authH := b.Auth`).

The harness that proves it: `router_table_test.go` pins all 294 routes with
their middleware chains in `testdata/route_table.golden`. Middleware is recorded
by **behaviour class** — `pass-through`, `optional-auth`, `auth-required`,
`role-required`, `rate-limited` — not by function name. On top of the golden file:

- `TestAdminRoutesRequireAuthAndAdminRole` — every `/api/admin/*` route carries
  both an auth gate and a role gate, so relocating the admin block cannot
  silently drop a privilege check.
- `TestCriticalRoutesAreMounted` — names eight endpoints whose absence is a
  security or availability failure, with the reason for each.
- `TestMissingDepsDoNotPanic` — states the property the router always relied on:
  a partially-wired `Deps` degrades an endpoint rather than killing startup.

**The harness broke in CI twice, and both failures are worth recording.**

The first version keyed on runtime symbol names. Go gives every closure returned
by one function the same runtime name *and the same code pointer* — verified by
probe: two closures from a factory report as `factory.func1` with equal pointers.
`AuthMiddleware`, `RequireRole` and the feature gate are all closures, so all
three collapsed to one token, and the admin assertion reported a privilege
escalation that did not exist. Separately, whether a substring like
`RealIPFromEnv` appeared at all depended on inlining, so the same source produced
a different golden string on the CI runner than on a laptop.

The second version probed behaviour but let the probe consume state. The rate
limiter keeps a shared token bucket with burst 200, and there are 294 routes, so
the 201st probe started receiving 429 and every route after it classified the
limiter as `rate-limited`. The golden recorded the artifact, and the boundary
moved with wall-clock refill and with `RATE_LIMIT_BURST`. **An instrument must
not mutate the system it measures.**

Each middleware is now probed with the limiter's burst raised for the
route-walking tests (`t.Setenv`, which also overrides a hostile ambient value),
so the limiter is a pure pass-through for the walk. Two probes remain, because a
role gate and an auth gate answer identically (401) to an anonymous request —
`RequireRole` checks claims before it checks the role. The second probe seeds
ordinary-user claims on the context, which is what `AuthMiddleware` leaves behind
in production, and a 403 there is the role gate identifying itself. The table is
now identical across runs, across `GOAMD64` levels, and under a hostile
`RATE_LIMIT_BURST=5`.

Both the golden diff and the admin assertion were confirmed to fail when
`RequireRole("admin")` is deleted from the new file, so the harness guards the
split rather than merely describing it.

### Finding 3 — `SubmissionStore`

The 28 methods are now five cohesive roles in `internal/store/interfaces.go`,
with `SubmissionStore` reduced to a pure union that embeds them (0 methods of
its own):

| Role | Methods | Consumers |
|---|---|---|
| `SubmissionReader` | 8 | listings and lookups |
| `SubmissionWriter` | 8 | create, verdict, rejudge, void |
| `SubmissionJudgeQueue` | 5 | local judge claim protocol |
| `SubmissionRemoteJudge` | 4 | vjudge hand-off |
| `SubmissionStats` | 3 | report queries |

**No consumer depends on the union any more.** Each now names what it uses:

| Consumer | Before | After |
|---|---|---|
| `judge/reclaimer.go` | 28 | 2 (`staleReclaimer`) |
| `hack/service.go` | 28 | 1 (`submissionLookup`) |
| `plagiarism/service.go` | 28 | 1 (`problemSubmissionLister`) |
| `handler/users.go` | 28 | 1 (`publicSubmissionLister`) |
| `handler/aggregator.go` | 28 | 2 (`submissionAggregateReader`) |
| `handler/contest_adjustment.go` | 28 | 2 (`adjustmentSubmissionReader`) |
| `judge/worker.go` | 28 | 3 (`submissionJudgeStore`) |
| `handler/stats.go` | 28 | 3 (`store.SubmissionStats`) |
| `vjudge/service.go` | 28 | 6 (`remoteSubmissionStore`) |
| `handler/admin_submissions.go` | 28 | 9 (`adminSubmissionStore`) |
| `handler/submission.go` | 28 | 20 (union of 3 roles) |

`handler/submission.go` is the one genuine breadth consumer, so its seam
**composes** the roles rather than restating them:

```go
type submissionServiceStore interface {
	store.SubmissionReader
	store.SubmissionWriter
	store.SubmissionRemoteJudge
}
```

The split found a gap the union had hidden: nothing had ever asserted that
`*postgres.SubmissionStore` satisfied each role separately, so a signature could
have drifted on any one of them. `internal/store/postgres/interface_assert_test.go`
now asserts all five.

Verified: `go build ./...` and `go vet` clean; `make lint` passes; handler
failures 12 and postgres failures 40 — both unchanged from baseline.

### Finding 6 — `main()` and the run modes

`main()` was 919 lines: the shared wiring every mode needs, then one inline block
per run mode, each ending with its own copy of the signal-wait-shutdown sequence.
The three non-server modes are now `runJudgeWorker`, `runAIWorker` and
`runAIHarness` in `cmd/aioj/modes.go`, each taking exactly what it uses.

The signal handling was identical in all three but the teardown was not, and the
teardown is the part that matters — a judge worker must drain, or cancelling alone
abandons in-flight judgments mid-write. `awaitShutdown` takes the teardown as a
callback so the wait is shared and the difference stays visible.

`main()` is 850 lines now, and the remainder is genuine wiring. Extracting the
modes also surfaced that `store.AIGenerationStore` was missing `SetManifest`, so
`problemgen.JobStore` was satisfied only by the concrete postgres type and
`main` had to pass it — the interface now declares it.

### Finding 7 — `Scoreboard`

320 lines, roughly two-thirds of which was the round's scoring policy. It is now
`format.BuildBoard`, a pure function of a `BoardInput`; the handler is 188 lines
doing fetch, ask, paginate, respond.

Two silent bugs surfaced during the move, and both are why the tests exist:

- The verdict was upper-cased in the handler because every format implementation
  compares `sub.Status == "AC"` exactly while the database stores `'ac'`. Dropping
  that made every submission look unsolved, so every score read zero.
- `Entry` and its cells had explicit JSON tags as a handler-local type.
  `format.Entry` had none, so Go would have emitted `Rank`/`UserID`/
  `TotalSolved` and the scoreboard would have rendered empty.

The freeze rule needed one deliberate choice: "is the clock past the freeze AND
before the end" is not "does the freeze fall inside the window", and the two
disagree for an ended round — where the second reading would withhold rows from
the one board that must show everything. `BuildBoard` takes `Now` explicitly.

### Finding 8 — recommendations

`GetRecommendations` was 159 lines on `ProblemStore`: four SQL queries plus the
policy deciding what the rows mean. The queries stay; the policy is
`internal/recommend`, a pure function with nine tests.

The move caught an ordering bug. The original applied `applyWeakTagDefaults`
*before* searching for weak-tag problems, because that search is driven by the tag
list. Defaulting only inside `Build` meant the search ran with the raw list, so a
user with no failed submissions — the user who most needs a recommendation — got
no weak-tag problems at all. The store's integration test caught it.

### Finding 10 — error responses

The two inline copies of the "this round needs its password" body in
`contest_access.go` had already drifted in what they included. Both are now
`contestPasswordRequired`, which derives its fields from the round, so a new
public surface is reported everywhere at once.

`errorBody` is a struct rather than a map because `scoreboard_public` and
`published_surfaces` are read by name in the SPA.

Not converted: the 1191 `http.Error` sites. The client already handles both shapes
(text fallback in `throwForStatus`), so a mechanical conversion is a very large
diff with no behavioural benefit. `TestConnection`'s 200-with-`ok:false` is
documented as a deliberate diagnostic the admin panel branches on, not an
oversight.

### Finding 12 — configuration

`main()` read eight environment variables directly while `config.Load` resolved
others. Four were in no config struct, absent from `config.yaml.example`, and
invisible to `Validate`: `FEED_TOKEN`, `CF_SUBMIT_URL`, `ATCODER_SUBMIT_URL`, and
`PUBLIC_ORIGIN`.

`PUBLIC_ORIGIN` was the worst: `Load` already resolved it into `Mail.PublicURL`,
but the sitemap and the link-preview cards re-read the variable at six call sites,
so setting `public_url` in yaml had no effect on them. An operator could configure
it correctly and watch it be ignored.

All four now live in `Config`, env as the override, previous defaults preserved.
`main()` has zero direct env reads.

### Finding 13 — `ContestManage` settings form

Measured rather than assumed: the "50 `useState`" in the first pass is spread
across twelve components, so the god-component framing was wrong. `SettingsTab`
alone was 768 lines, and that is what got split.

`AccessTable` and `FormatSettings` are now in `components/ContestManage/`,
following the `components/SetterWorkspace/` precedent. State stays in the parent —
both are one field of a form saved as a single PUT, so lifting it would have split
one save into two.

Ten tests pin the behaviour that has been wrong before: clearing a surface removes
the key rather than storing a blank audience, submit is not offered "participants
only" (requiring an entrant to submit deadlocks the first submission), an unknown
format says its settings are untouched rather than rendering an empty fieldset
that a save would clear, and a stored value the registry no longer knows is still
rendered so a save cannot silently drop it.

### Finding 3 — the rest of the stores

After `SubmissionStore` came `ContestStore` (31 → 5 roles), `UserStore`
(21 → 4), and `ProblemStore` (21 → 4). Each is a union of embeds now, so the wide
name still works for the one consumer that spans it.

Measured before carving, every time. For `ProblemStore`: nineteen consumers, twelve
using one to three methods, only the problem handler spanning the set. Those twelve
moved onto seams sized to what they call, and `canReadProblem` /
`canSeeSpoilers` / `problemIsLive` — which took the wide interface while using
three methods between them — moved onto `problemVisibility`.

Two things surfaced that the compiler could not catch, because a concrete type
always satisfies a narrower interface:

- Six handler-called methods were declared on no interface at all
  (`GetScoreboardRows`, `GetContestStats`, `RegisterTeam`,
  `ListTeamRegistrations`, `GetByIDForViewer`, `ListUsersByRating`). A test double
  could not have implemented them.
- `SetManifest` was missing from `AIGenerationStore`, so `problemgen.JobStore` was
  satisfied only by the concrete postgres type and `main` had to pass it.

Role assertions in `interface_assert_test.go` went from 8 to 22, one per role. They
are not ceremony: at several intermediate points the build passed while a role had
stopped being satisfied, and only the assertions said so.

### Finding 3 — `AIGenerationStore`, and where to stop

`AIGenerationStore` (20) had three consumers with three shapes, so it became
`AIGenerationAdminStore` (the HTTP surface) and `AIGenerationJobStore` (the
worker's claim protocol, including the per-actor limit the job service enforces
before claiming). `main`'s harness mode now takes `problemgen.JobStore`, the
further-narrowed view that already existed and belongs to the harness.

Two one-method consumers got seams: the profile's `GetUserPendingInvites` out of
19 `GroupStore` methods, and `users.go`'s group field.

**Where this stops.** `TrainingPlanStore` (21) has exactly one consumer using 16
methods, and `group.go` uses 16 of 19. Splitting those would add a layer without
removing a dependency — the definition of a carve that is not worth doing. The
measurements are what say so, which is why every one of these started with a
consumer survey rather than a method count.

## Still open

| Finding | What remains |
|---|---|
| 1 | Closed. `NewRouter` is 58 lines plus 16 mount files; `Deps` separates required fields from `Deps.Optional`; and an unwired required handler fails at boot. |
| 13 | `ContestManage` 1880 and `ContestDetail` 1123, both with the shared fetch hook and an extracted tab. `Profile` (1226), `ContestProblem` (1072) and `ProblemDetail` (877) remain, but each is already built from 12+ shared modules — long because it renders a lot, not because it is tangled. |

Closed since the first pass: 3 (every store that had more than one consumer),
4 (vjudge), 6 (run
modes), 7 (scoreboard), 8 (recommendations), 9 (the real half), 10 (contest
refusal), 11 (test stubs), 12 (config), 13 (settings form), 14 (false positive).

### Finding 13 — the shared fetch hook

The duplication half: eleven tab components open-coded the same data/loading/load/
useEffect scaffold, and each handled failure differently — some did not catch at
all, two used an empty catch, so a failed fetch was a permanent spinner on one tab
and an empty list on another.

`useAsyncData` is the shape they all wanted. Five tabs use it now, and
ContestManage drops 11 `useState` and 5 `useEffect`. Two behaviours the
hand-rolled versions got wrong are pinned: a superseded request cannot overwrite
a newer result (verified by removing the guard and watching the test fail), and
`reload` marks loading immediately.

The `SubmissionStore` split (finding 3) is the worked example for the remaining
interface carves: measure the consumers first, then carve along the axes the
measurements show. The router split (finding 1) is the worked example for moving
code that nothing else can check — write the equivalence harness first, confirm
it fails when you break something, then move the code.
