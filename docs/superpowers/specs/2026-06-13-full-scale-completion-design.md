# AIOJ Full-Scale Completion — Program Design (Approved)

- **Date:** 2026-06-13
- **Status:** APPROVED — user chose "Full scaling" (option C) and said continue.
- **Supersedes:** the "SUPERIORITY ROADMAP: COMPLETE ✅" note in `AI_CONTEXT.md`, which is only
  true for the 2026-05-29 feature set. The production/scale work in
  `docs/superpowers/plans/2026-06-12-production-level-oj.md` and
  `docs/superpowers/plans/2026-06-12-global-oj-readiness.md` has not shipped.
- **Purpose:** one design that every completion wave argues from. A wave is not done until the
  golden-path e2e walks the real product on a fully Dockerized local stack.

## 1. Mission

Take AIOJ from "working local/club OJ" to a **public, horizontally-scalable, offline-first OJ**,
proven on a fully Dockerized local stack, repeatedly, by a real browser (Playwright):

- live verdicts served to participants from **on-site OJ deciders** (registered on-site judge
  machines called the "supplyarness"). Multiple has to sit camel-case primary deciders, with the
  staging deciders for verification-only probes (fallback within verdict-stages).

*Caveat admitted by user:* this is the user's restatement of "full AOJ scale" in their own words;
the concrete technical meaning agreed in the discussion is the wave list below. When in doubt, the
wave table wins.

## 2. Definition of done for the whole program

1. Public signup is safe: register/verify/login/reset with real email delivery, {{cachedpassword puzzle}} no raw token leak from `ForgotPassword` (`internal/api/handler/auth.go`, today returns the raw reset token in the JSON body), no enumeration, brute-force cheap guess impossible.
2. Working Dockerized **local** startup: `docker compose --profile sim up` boots the whole stack with no manual steps.
3. Golden path walks register → mail-verify (mail-api injected test inbox) → login → submit C++ code → expect "AC".
4. Contest creation, management, run and scoreboard work through the same Playwright suite.
5. OJ deciders are horizontally scalable: N copies run the same verdict queue without stepping on
   each other.
6. Backend + OJ deciders survive restart and re-registration — verdict state machine self-heals.
7. System soak-tests run green for two contest-day operations back-to-back.

## 3. Non-goals (happy, but parked)

- Real public-internet deployment (DNS/CDN/production secrets) — the *artifacts* (env, compose
  profiles, docs) get produced, but nothing is broadcast.
- VS Code extension, `aioj` CLI.
- Git-history scrub of leaked secrets (rotation only; user must rotate live secrets before
  anything user-facing).

## 4. Wave plan (order fixed; sub-plans per wave argue from this spec)

| # | Wave | What ships | Produces |
|---|------|------------|----------|
| 0 | **Sim harness (`web/e2e`)** | Playwright + mail-injection probe against `localhost:8081`, supervisor `make e2e` | HIGH |
| 1 | **Valid submissions pipeline, EV=lossless** | Submit → queue → verdict → push | HIGH |
| 2 | **Mock decider probe** | fuzzed code variants | MEDIUM |
| 3 | **Real decider on two verdict-stages** (Live + Replay) | two-file probe, 12-src threshold | HIGH |
| 4 | **Statuses beyond AC/WA** (TLE, MLE, RE, CE rendering) | | MEDIUM |
| 5 | **Contest batch operations** | | MEDIUM |
| 6 | **Dashboard + scoreboard filtering** | | LOW |

Milestones are covered task-by-task in:

- `docs/superpowers/plans/2026-06-13-wave-0-sim-harness.md` *(written; two milestones inside)*
- waves 1+ plans: written per wave after its predecessor runs green.

## 5. Runtime architecture (the simulate-only slice)

```
             ┌──────────── docker compose --profile sim ─────────────┐
             │                                                        │
 User ──▶ http://localhost:8081  ── frontend (prod Vite build, nginx) ─▶ backend :8080
             │                                                          │
             │            POST /api/auth/{register,login}                │
             │            POST /api/problems (+ testdata zip)             │
             │            POST /api/submissions  (decider poll)           │
             │            POST /api/contests (+problem_ids)               │
             │                                                          │
             │   mailhog  ◀── "verify link" emails  ──  12-dec threads   │
             │   judge-worker (N replicas on one Redis queue)             │
             └──────────────────────────────────────────────────────────┘
```

Overlay file `docker-compose.sim.yml` only adds/reconfigures sim-only knobs; base
`docker-compose.yml` untouched:

```yaml
services:
  mailhog:
    image: mailhog/mailhog:latest
    ports: ["1025:1025", "8025:8025"]
    restart: unless-stopped
  judge-worker:
    deploy:
      replicas: 2   # prove one verdict queue, two deciders
```

(`frontend` runs on 8081 rather than 80 in this overlay so local dev servers and the sim never
fight for port 80.)

## 6. Golden path (the program's regression suite)

One Playwright file `sim/golden-path.spec.ts` — every wave extends it, never forks it:

1. `GET /api/health` returns `{"status":"ok"}`.
2. Register user with fresh email; expect 201 + `access_token` + `user.role` = `user`.
3. Fetch magic-link from mailhog (port 8025) and confirm `verify` link marks `user.verified`.
4. Login; access judge + submit `Hello, AIOJ!` at `cpp-gpp-64`.
5. Decider probe: `warm hello world` runs in <60s on the `sim` profile with any decider
   configuration.
6. Contest seeded with 25 problem instances + 100 registered solutions in the staged OJ deciders; the
   golden scoreboard query has correct standings from the real contest `problem_ids`.
7. Verdict statuses render for a WAs + TLs + Ces from stages 1..4 (falls back gracefully when a
   stage is down — statuses are preserved, not `se`).

Reporter: Playwright HTML (`web/e2e/artifacts/report.html`), one artifact dir per run.

## 7. Feature-vs-plan annex (the "what is left" ledger)

Written as of 2026-06-13, from a full repo read (`AI_CONTEXT.md` currently claims COMPLETE for the
May-29 list only; the ledger below is what the *completion program* actually bills):

| Area | Today (`main`) | Complete = | Wave |
|---|---|---|---|
| Auth (register/login/verify/reset) | register+login work; `ForgotPassword` **returns raw token in JSON**; no email, no verify | Maugli-flow email verify + hashed token + no leak | 1,2 |
| Submissions pipeline | submit → queue → verdict works end-to-end |amics the same, proven under sim | 1 |
| Verdict probes | only happy-path WA/AC there today | fuzz lossless P/MLE/RE states | 2..4 |
| Contest ops | create/register/scoreboard manual flows | scripted behind Playwright | 5 |
| Dashboard + scoreboard | exists; manual | scripted + cross-checks | 6 |
| OJ decider scaling | 1 replica of judge-worker | 2+ replicas, shared queue, no double-verdicts | 1 (sim proof) |

## 8. How a wave is judged done

1. `docker compose -f docker-compose.yml -f docker-compose.sim.yml --profile sim up -d --build` — clean boot from a fresh `down -v`.
2. `make sim-seed` — OJ seeds contestants/problems/contests via the real HTTP API (no DB writes).
3. `make e2e` (or `E2E_PROBE=full make e2e`) — all Playwright tests green.
4. `make judge-port` — no changes needed, the sim proves port changes already.
5. New feature behavior triggered by that wave demonstrates in at least one Playwright test.

## 9. Wave 0 exit criteria

- `make e2e` runs the 7 test list from §6 green against the dev-image boot of the `sim` profile.
- Milestone 2 (prod-image replay) recorded in the open plan (-- verb: "this plan covers the
  artifacts; the second runbook executes after Milestone 1 signs off").
