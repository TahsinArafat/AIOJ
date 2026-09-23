# Phase C — Global Reach (i18n, CDN, Multi-region)

**Status:** Implemented (2026-06-13) — core code paths; ops items documented as runbooks.

## Tasks

| # | Item | Status | Notes |
|---|------|--------|-------|
| 1 | Locale files `zh`,`ru`,`ja`,`es` | ✅ | Full key parity with `en` (90 keys); registry-driven |
| 2 | Translation pipeline | 📄 | One-way: edit `en.json` master → mirror to others (parity test enforces) |
| 3 | RTL readiness | ✅ | `isRtl` + `<html dir>` from `languageChanged`; set `rtl: true` on future ar/he |
| 4 | Per-locale / asset caching | ✅ | `deploy/Caddyfile`: immutable `/assets/*`, `no-cache` index, 7d `/media/*` |
| 5 | CDN (Cloudflare free) | 📄 | Point DNS at Caddy; enable CF proxy; see runbook |
| 6 | Per-problem statement i18n | ✅ | Already shipped: `problem_i18n` store/handler + TranslationsTab |
| 7 | CDN asset pipeline | 📄 | Problem images → R2/S3 later; current: `/media` + long cache |
| 8 | Multi-region runbook | ✅ | `docs/runbooks/multi-region.md` |
| 9 | Geolocation feature flags | ✅ | `features.remote_bots_regions` + env override (empty = all) |
| 10 | Status page | 📄 | Deferred: BetterUptime / CF Workers (ops) |
| 11 | Domain policy | ✅ | `docs/runbooks/domain-policy.md` |
| 12 | CDN cache invalidation API | ✅ | `POST /api/admin/cdn/purge` (Cloudflare zone purge) |

## Key files

- `web/src/i18n/locales/{zh,ru,ja,es}.json` (new)
- `web/src/i18n/locales/index.ts` (registry + `isRtl`)
- `web/src/i18n/index.ts` (sync `lang` + `dir`)
- `web/src/i18n/locales/locales.test.ts` (parity for every locale)
- `deploy/Caddyfile` (cache headers)
- `internal/config/config.go` (`FeaturesConfig`)
- `internal/api/handler/cdn.go` + router `/api/admin/cdn/purge`
- `docs/runbooks/multi-region.md`, `docs/runbooks/domain-policy.md`

## Verification

- `vitest locales` — all LOCALES parity with en
- `go test ./...` / `go build ./...`
- `tsc -b` / `npm run build`
