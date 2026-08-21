# AIOJ Project Memory

## Security decisions (2025 code-smell fix rounds)
- `config.yaml` untracked (real DB password + JWT secret were committed); template in `config.yaml.example`; secrets rotatable via `DB_PASSWORD`/`JWT_SECRET` env vars — **user still must rotate exposed values; they remain in git history**.
- `aioj-linux` (47MB) and `vjudge-test` (21MB) untracked via `.gitignore`. History not scrubbed.
- `calculate-ratings`: canonical route `POST /api/contests/{id}/calculate-ratings` inside AuthMiddleware group; duplicate `/api/rating/calculate/{id}` removed (frontend ContestScoreboard.tsx repointed). Handler enforces admin internally too.
- `POST /api/contests/{id}/register-team`: auth middleware + **captain-only** check via `store.TeamStore.GetMemberRole(teamID, uid) == 'owner'` (roles: owner/member/invited/requested).
- Backup filenames all go through `sanitizeFilename()` in admin_backup.go; uploads report real on-disk stat; failed `cp` fallbacks log via slog through `copyDir` helper.
- **Round 2 (done):**
  - `GET /api/submissions/{id}` now calls existing `hasSubmissionAccess` (owner/admin/contest manager/judge) → 403 before serializing. Public `ListByProblem` (`/api/problems/{slug}/submissions`) was always safe — store query projects no source_code.
  - `CustomRun` (`POST /api/submissions/run`) client limits clamped to ceilings 10s / 512MB (2× defaults).
  - `?token=` query-param JWT now accepted **only** on backup download route `GET /api/admin/backups/{filename}` via new `QueryTokenAuthMiddleware`; global `AuthMiddleware` is header-only. BackupsPanel.tsx downloads unaffected.
  - vjudge verdict dedup: `mapCFVerdict` (with MLE) + `mapGenericVerdict` (no MLE—default WA, preserving prior behavior) in service.go; 4 inline switches removed.

## Codebase facts
- `store.TeamStore` interface exposes GetMemberRole/IsMember — depend on interface.
- Test DB drift: migration 000052 `ai_generated` not applied → 3 pre-existing integration failures (TestGetRecommendationsDB, TestTrainingPlanStore_*). Same failures with changes stashed.
- VS Code format-on-save reflows adjacent template-literal whitespace in touched .tsx files; harmless reflow hunks may appear.
- vjudge remaining nits (deferred): `bot.Configure(cfg)` mutates shared bot without lock; poll workers use context.Background() children (benign today).
- Rate limiter keyed on r.RemoteAddr only (no X-Forwarded-For handling) — deferred.

## Deferred work (agreed with user)
- Frontend smell batch: 23 hand-decoded `atob(token.split('.')[1])` across 18 files; **101 alert() calls across 32 files** (~30 `catch (e: any)`); api.ts = 761 lines / ~173 `any`s; 6 near-identical importX() functions in api.ts; zero AbortController; ContestManage.tsx 14 effects/0 cleanups; no ErrorBoundary; no react-query/SWR; hardcoded 130-country list.
- Judge sandbox config-driven limits deserve explicit isolate/network/resource review (executor = external go-judge HTTP service).
