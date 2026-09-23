# Multi-region deployment runbook

**Goal:** Low latency for target markets; single primary Postgres with optional read replica; judges close to users.

## Region selection matrix

| Audience | Primary region | Edge/CDN | Notes |
|----------|----------------|----------|-------|
| South Asia (BD, IN) | Hetzner Nuremberg or Falkenstein | Cloudflare free | Prefer EU for cost; CF PoP in Dhaka/Delhi |
| Europe | Hetzner Nuremberg | Cloudflare | GDPR-friendly storage |
| North America | DigitalOcean / Vultr NYC or SJC | Cloudflare | |
| East Asia | Vultr Tokyo / Osaka | Cloudflare | Latency-sensitive; consider CDN-only first |

**Recommendation:** Start single-region (Hetzner EU) + Cloudflare global edge. Add second app region only when p95 judge queue or page TTFB demands it.

## Topology (target)

```
Users → Cloudflare (TLS, cache, WAF)
         → Caddy (origin TLS, cache headers)
              → frontend (SPA)
              → api (chi)
                   → Postgres primary (+ read replica)
                   → Redis
                   → judge-worker pool (same region as primary)
```

## Checklist for a new region

1. Provision VM; install Docker; open 22/80/443 only.
2. Clone repo; `make docker-up` (or compose staging).
3. Set `PUBLIC_ORIGIN`, secrets via env (never commit).
4. Run migrations: `make migrate-up`.
5. Point a CF origin pool / subdomain at the region.
6. Warm judge: scale `judge-worker` replicas; pre-pull `lang/` images.
7. Verify: `/api/health`, sample submit, scoreboard latency.

## Read replica (Phase E-ready)

- Config key: `database.replica_dsn` (planned).
- Route: analytics, rankings reads, public problem list.
- Keep writes on primary.

## Failover (manual MVP)

1. Promote replica (or restore from nightly base backup + WAL).
2. Flip `DB_*` / DSN on app; `docker compose up -d`.
3. Update CF origin to healthy pool.
4. Run `make migrate-status`.

## Related

- Domain policy: `docs/runbooks/domain-policy.md`
- Disaster recovery: `docs/runbooks/disaster-recovery.md` (Phase E)
