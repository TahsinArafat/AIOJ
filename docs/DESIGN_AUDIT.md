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

### 1. `Deps` is a service locator and `NewRouter` was a second program — **High** (router split fixed)

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
still encoded as `if d.X != nil` across 31 sites rather than in the type.
Grouping `Deps` into required and optional sub-structs remains the fix.

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

### 3. `SubmissionStore` and friends are far wider than any consumer — **High**

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

### 13. God page components — **Medium**

`web/src/pages/ContestManage.tsx` — **2062 lines, 50 `useState`, 14 `useEffect`**,
holding 19 stateful sub-components in one file (`SettingsTab` 8 state,
`ModeratorsTab` 6, `SubmissionsTab`, `OnsiteTeamsTab`, `StandingsTab`,
`StatisticsTab`, `BalloonsTab`, `AnnouncementsTab`, `PrintsTab`,
`ParticipantsTab`, `ClarificationsTab`). 18 `await api.*` calls and 41
`setLoading`/`setError` sites with no shared fetch hook.

`ContestDetail.tsx` (1204), `Profile.tsx` (1226), `ContestProblem.tsx` (1017),
`ProblemDetail.tsx` (853) have the same shape.

### 14. `With*` mutators as a second constructor — **Low**

**21** `With*` methods mutate a handler after construction; 5 call
`rebuildWithGrants`, re-deriving the same access policy. Confidence `this` is
discarded, so the fluent form misleads at a type level.

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

**The first version of this harness keyed on runtime symbol names and broke in
CI**, which is worth recording because the failure looked like a security bug. Go
gives every closure returned by one function the same runtime name *and the same
code pointer* — verified by probe: two closures from a factory report as
`factory.func1` with equal pointers. `AuthMiddleware`, `RequireRole` and the
feature gate are all closures, so all three collapsed to one token, and the admin
assertion reported a privilege escalation that did not exist. Separately, whether
a substring like `RealIPFromEnv` appeared at all depended on inlining, so the same
source produced a different golden string on the CI runner than on a laptop.

Each middleware is now probed instead: serve a request through it and read what
it did. Two probes, because a role gate and an auth gate answer identically (401)
to an anonymous request — `RequireRole` checks claims before it checks the role.
The second probe seeds ordinary-user claims on the context, which is what
`AuthMiddleware` leaves behind in production, and a 403 there is the role gate
identifying itself. The table is now identical across runs and across `GOAMD64`
levels.

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

## Still open

| Finding | What remains |
|---|---|
| 1 | Findings 4 and 11 are closed. `Deps` is still a 77-field service locator and optionality is still `if d.X != nil` rather than a type; the route split is done. |
| 3 | Still owed to `ContestStore`, `UserStore`, `ProblemStore`, `TrainingPlanStore`, `GroupStore`, `AIGenerationStore`. |
| 6 | `main()` is still 920 lines across four run modes. |
| 7 | Business rules still live in the HTTP layer (`Scoreboard`, 320 lines). |
| 8 | Report logic still lives in the postgres layer (`GetRecommendations`). |
| 10 | Error response shapes are still four-way inconsistent. |
| 12 | Config is still read three ways. |
| 13 | God page components are untouched. |
| 14 | `With*` mutators still form a second constructor. |

The `SubmissionStore` split (finding 3) is the worked example for the remaining
interface carves: measure the consumers first, then carve along the axes the
measurements show. The router split (finding 1) is the worked example for moving
code that nothing else can check — write the equivalence harness first, confirm
it fails when you break something, then move the code.
