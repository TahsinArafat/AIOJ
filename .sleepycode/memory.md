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
  - vjudge verdict dedup: `mapCFVerdict` (with MLE) and `mapGenericVerdict` (no MLE—default WA, preserving prior behavior) in service.go; 4 inline switches removed.

## Codebase facts
- `store.TeamStore` interface exposes GetMemberRole/IsMember — depend on interface.
- Test DB drift: migration 000052 `ai_generated` not applied → 3 pre-existing integration failures (TestGetRecommendationsDB, TestTrainingPlanStore_*). Same failures with changes stashed.
- VS Code format-on-save reflows adjacent template-literal whitespace in touched .tsx files; harmless reflow hunks may appear.
- vjudge remaining nits (deferred): `bot.Configure(cfg)` mutates shared bot without lock; poll workers use context.Background() children (benign today).
- Rate limiter keyed on r.RemoteAddr only (no X-Forwarded-For handling) — deferred.

## Gap ledger (global readiness)
- **Slice A mail + reset + email verify (2026-06-13): DONE** — plan `docs/superpowers/plans/2026-06-13-slice-a-trust-completion.md`. Shipped: register JWT test fix; submit gate `requireVerifiedEmail` on Create/CreateUpsolving/CustomRun (403); onsite users auto-`MarkEmailVerified`; `POST /api/auth/verify-email/resend` (auth, enumeration-safe); frontend `/verify-email` page + Register check-email + api helpers. Verified: `go test ./internal/api/handler ./internal/mail`, `go build ./...`, `npx tsc -b --noEmit`, `npm run build`. Live stack may still run old binary until restart (resend 404 until rebuild). Commits deferred pending user approval.
- **Slice A committed:** `1a73f25` feat(mail)…
- **Slice 2FA + password policy (A.12–A.16): DONE** — migration `000060_totp` (plan said 000053; renumbered because 000053=ai_models). TOTP begin/enable/disable + challenge login `POST /api/auth/2fa/verify`, 12-char complexity policy on Register/ResetPassword/UpdatePassword, Login UI for 2FA + Register min-12 hint. Verified: `go build`, `go test ./internal/auth ./internal/api/handler ./internal/mail`, `tsc -b --noEmit`. Next: Phase A A.17–A.22 OAuth.
- **Security middleware (A.23–A.25): DONE** — CSRF double-submit (Bearer exempt), SecurityHeaders, StrictAuth rate limit on auth POSTs, CSRF_SECRET config. Verified middleware/handler/auth/mail tests + tsc. Next: A.17–A.22 OAuth or A.26+ Sentry.
- Already present before this slice (do not re-do): `internal/mail` package, ForgotPassword email (no token in response), register verification email, `GET /api/auth/verify-email/{token}`, ForgotPassword/ResetPassword pages, migration 000059 email_verified, DevMail inspector.

- **OAuth (A.17–A.22): DONE** — `internal/oauth` state+GitHub+Google, migration `000061_oauth` (plan said 000052; renumbered), start/callback handlers (SPA fragment tokens via `/oauth/complete`), Login SSO buttons, config `oauth:` + env overrides. Verified: `go build`, `go test ./internal/oauth ./internal/api/handler …`, `tsc`. Next: A.26+ Sentry or legal pages A.28.

## Deferred work (agreed with user)
- Frontend smell batch: 23 hand-decoded `atob(token.split('.')[1])` across 18 files; **101 alert() calls across 32 files** (~30 `catch (e: any)`); api.ts = 761 lines / ~173 `any`s; 6 near-identical importX() functions in api.ts; zero AbortController; ContestManage.tsx 14 effects/0 cleanups; no ErrorBoundary; no react-query/SWR; hardcoded 130-country list.
- Judge sandbox config-driven limits deserve explicit isolate/network/resource review (executor = external go-judge HTTP service).
