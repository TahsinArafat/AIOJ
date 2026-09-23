Next: A.31 cookie consent banner.
# AIOJ Project Memory

## Security decisions (2025 code-smell fix rounds)
- `config.yaml` untracked (real DB password + JWT secret were committed); template in `config.yaml.example`; secrets rotatable via `DB_PASSWORD`/`JWT_SECRET` env vars — **user still must rotate exposed values; they remain in git history**.
- `aioj-linux` (47MB) and `vjudge-test` (21MB) untracked via `.gitignore`. History not scrubbed.
- `calculate-ratings`: canonical route `POST /api/contests/{id}/calculate-ratings` inside AuthMiddleware group; duplicate `/api/rating/calculate/{id}` removed (frontend ContestScoreboard.tsx repointed). Handler enforces admin internally too.
- `POST /api/contests/{id}/register-team`: auth middleware + **captain-only** check via `store.TeamStore.GetMemberRole(teamID, uid) == 'owner'` (roles: owner/member/invited/requested).
- Backup filenames all go through `sanitizeFilename()` in admin_backup.go; uploads report real on-disk stat; failed `cp` fallbacks log via slog through `copyDir` helper.
- **Round 2 (done):**
  - `GET /api/submissions/{id}` now calls existing `hasSubmissionAccess` (owner/admin/contest manager/judge) → 403 before serializing. Public `ListByProblem` (`/problems/{slug}/submissions`) was always safe — store query projects no source_code.
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
- **Slice 2FA + password policy (A.12–A.16): DONE** — migration `000060_totp` (plan said 000053; renumbered because 000053=ai_models). TOTP begin/enable/disable + challenge login `POST /api/auth/2fa/verify`, 12-char complexity policy on Register/ResetPassword/UpdatePassword, Login UI for 2FA + Register min-12 hint. Verified: `go build`, `go test ./internal/auth ./internal/api/handler ./internal/mail`, `tsc -b --noEmit`. Commit: `e1c6d0e`.
- **Security middleware (A.23–A.25): DONE** — CSRF double-submit (Bearer exempt), SecurityHeaders, StrictAuth rate limit on auth POSTs, CSRF_SECRET config. Commit: `77703e6`. Next: A.17–A.22 OAuth or A.26+ Sentry.
- **OAuth (A.17–A.22): DONE** — `internal/oauth` state HMAC + GitHub/Google, migration `000061_oauth` (plan said 000052; renumbered), start/callback handlers (SPA fragment tokens via `/oauth/complete`), Login SSO buttons, config `oauth:` + env overrides (OAUTH_STATE_SECRET, GITHUB_*, GOOGLE_*). Commits: `35ce76b` + `5fdb25c`. Verified: `go build`, `go test ./internal/oauth ./internal/api/handler ./internal/api/middleware ./internal/auth ./internal/mail`, `tsc`. Next: A.26+ Sentry or legal pages A.28.
- Already present before this slice (do not re-do): `internal/mail` package, ForgotPassword email (no token in response), register verification email, `GET /api/auth/verify-email/{token}`, ForgotPassword/ResetPassword pages, migration 000059 email_verified, DevMail inspector.

- **Sentry (A.26–A.27): DONE** — `internal/observability.InitSentry` (SENTRY_DSN no-op when empty), `middleware.SentryRecover` before chi Recoverer, `web/src/lib/sentry.ts` + `initSentry()` in main.tsx (`@sentry/react` installed with --legacy-peer-deps due to katex/tiptap peer conflict). Env: SENTRY_DSN, SENTRY_ENVIRONMENT, AIOJ_RELEASE, VITE_SENTRY_DSN, VITE_AIOJ_RELEASE. Verified: go build/test/vet + tsc. Next: A.28 legal pages.
- **Legal pages (A.28): DONE** — `docs/legal/{TERMS_OF_SERVICE,PRIVACY_POLICY,DMCA}.md` (placeholder stubs, lawyer review still required) + embed copies under `internal/api/handler/legal/`. `GET /api/legal/{doc}` allowlist (`terms_of_service|privacy_policy|dmca`) with traversal rejection. SPA: `web/src/pages/legal/{LegalDoc,TermsOfService,PrivacyPolicy,DMCA}.tsx` + routes `/legal/terms|privacy|dmca` + footer links. CookieConsent `/legal/privacy` works. Verified: `go build`, `go test ./internal/api/handler`, `npx tsc -b --noEmit`. Next: A.29 GDPR export.
- **GDPR export (A.29): DONE** — `GET /api/users/me/export` (auth) via `UsersExportHandler` + `userDataAggregator` (user, profile, submission_count; password_hash json:"-"). Profile “Download My Data” → `aioj-data-export.json`. Tests: export JSON + 401 without claims. Verified: go build/test, tsc. Next: A.31 cookie consent banner.
- **Account deletion (A.30): DONE** — DELETE /api/users/me with password re-auth + confirm=true; migration 000062 (personal CASCADE, shared authors SET NULL); UsersDeletionHandler + UserCascadeDeleter; Profile Danger Zone UI. Tests: 403 wrong password, 400 no confirm, 200 happy, 401. Verified: go build/test, tsc. Next: A.31 cookie consent banner.

## Deferred work (agreed with user)
- Frontend smell batch: 23 hand-decoded `atob(token.split('.')[1])` across 18 files; **101 alert() calls across 32 files** (~30 `catch (e: any)`); api.ts = 761 lines / ~173 `any`s; 6 near-identical importX() functions in api.ts; zero AbortController; ContestManage.tsx 14 effects/0 cleanups; no ErrorBoundary; no react-query/SWR; hardcoded 130-country list.
- Judge sandbox config-driven limits deserve explicit isolate/network/resource review (executor = external go-judge HTTP service).
