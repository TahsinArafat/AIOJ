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

### 1. `Deps` is a service locator and `NewRouter` is a second program — **High**

`internal/api/deps.go:14`, `internal/api/router.go:16`

`Deps` has **77 fields**. `NewRouter` is **893 lines** — longer than any file in
`internal/store/` — and does four jobs: unpack dependencies (50 `xH := d.X`
aliases, lines 17–66), build the feature-gate closure, assemble middleware, and
register all 408 routes.

Optionality is encoded as nil rather than in the type: handlers require roughly
40 of the 77 fields and the rest are guarded by `if d.X != nil` across **31
sites**. Nothing prevents launching a half-wired server.

*Direction:* split route registration into per-domain `func mount<Domain>(r chi.Router, d Deps)` files; group `Deps` into required and optional sub-structs so wiring gaps are type errors.

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

### 4. The remote-judge open/closed design is bypassed where it exists — **High**

`internal/vjudge/bot.go:25` defines a proper 10-method `Bot` interface with
`RegisterBot`. Then `internal/vjudge/service.go` special-cases one platform
inline: `service.go:186` (remote URL), `:349` (`ForcePoll` hand-rolls CF
scraping), `:472` (`if platform == "codeforces" { pollCF } else { pollGeneric }`).
Because `pollCF` bypasses `Bot`, the one backend that *is* an interface is also
the one reached without it.

Cost of the same defect elsewhere:

- `handler/admin_bot_accounts.go:228,268` — credential validation is an if/else
  over platform names with different semantics per platform.
- `cmd/aioj/main.go:304` — `switch platform` with 5 hardcoded constructors;
  a 6th platform means editing `main.go`.

Contrast `internal/contest/format/registry.go`, which is textbook open/closed:
`Register(name, factory)` plus `init()` self-registration. Adding a scoring
format touches nothing shared.

*Direction:* give `Bot` a `RemoteURL`/`Poll` seam to delete the three branches,
and mirror the format registry for bots.

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

### 9. Errorless store contracts fail silently — **Medium**

`internal/store/interfaces.go` — `HasAccess(...) bool` (×2, lines 104, 262),
`GetUsername(...) string` (line 254).

`postgres/contests.go:545` runs `QueryRow(...).Scan(...)` and **discards the
error**, returning `""` for both "no such user" and "database down". A caller
cannot tell the two apart.

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

Findings 1, 4, 6–14, plus the same carve for `ContestStore`, `UserStore`,
`ProblemStore`, `TrainingPlanStore`, `GroupStore` and `AIGenerationStore`. The
`SubmissionStore` split above is the worked example to copy: measure the
consumers first, then carve along the axes the measurements show.
