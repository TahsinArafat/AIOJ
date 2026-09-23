# Domain policy

| Domain | Purpose | Redirect / notes |
|--------|---------|------------------|
| `aioj.com` | Primary (English default, i18n via UI switcher) | Canonical |
| `www.aioj.com` | Alias | 301 → `https://aioj.com` |
| `app.aioj.com` | Optional app host | Same origin as API if single-box |
| `api.aioj.com` | Optional API split | CORS: allow `aioj.com` origins only |
| `aioj.cn` | Future CN mirror (Phase C stretch) | Separate deploy + CN CDN; not proxied from primary |
| `status.aioj.com` | Status page | CF Workers / BetterUptime |
| `changelog.aioj.com` | Public roadmap | Static (Astro) — Phase D |

## Rules

1. **HTTPS only** — HSTS preloaded via Caddy/CF; HTTP → HTTPS 301.
2. **One canonical host** — non-canonical hosts redirect permanently.
3. **Locale in UI, not in host** — language switcher + `Accept-Language`; avoid `/bn/...` path prefixes unless SEO requires it later.
4. **Cookies** — `__Host-` prefixed where possible; SameSite=Lax; consent banner already ships (A.31).
5. **Email links** — always built from `mail.public_url` / `PUBLIC_ORIGIN` (no localhost in prod).

## Cloudflare

- DNS: `A`/`CNAME` proxied for apex + www.
- Cache rules: `/assets/*` long, `/index.html` bypass, `/api/*` bypass cache.
- Purge: `POST /api/admin/cdn/purge` or CF dashboard.
