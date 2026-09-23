# Disaster recovery runbook

**RPO target:** ≤ 15 min (WAL / continuous archive) · **RTO target:** ≤ 60 min

## Blast radius

| Scenario | Impact | First action |
|----------|--------|--------------|
| Single VM loss | Full outage | Restore from image + latest base backup |
| Postgres corruption | Data plane down | Promote replica or restore PITR |
| Accidental DROP | Partial data | PITR to timestamp before drop |
| Region outage | Multi-service | Failover DNS/CF origin (see multi-region.md) |

## Nightly backup (existing admin backup UI)

1. Admin → Backups → Create (uses `./backups` + `pg_dump`).
2. Copy off-box: `rsync -av ./backups/ backup-host:/aioj/backups/`.
3. Verify: restore into a scratch DB weekly (`make migrate-up` then restore dump).

## PITR (recommended)

```bash
# base
pg_dump -Fc -f /backups/base-$(date +%F).dump "$DB_DSN"
# WAL archive configured in postgres (archive_mode=on)
```

## Restore procedure (primary)

1. Stop writers: `docker compose stop backend judge-worker`.
2. Restore:
   ```bash
   pg_restore --clean --if-exists -d "$DB_DSN" /backups/base-YYYY-MM-DD.dump
   ```
3. Migrate: `make migrate-up` (must be ≥ schema of the dump).
4. Start: `docker compose up -d backend judge-worker`.
5. Smoke: `GET /api/health` → `status=ok`; login; one submit.

## Failover to replica

1. Promote: `pg_ctl promote /var/lib/postgresql/data`.
2. Point `DATABASE_*` (or `DB_*`) at the promoted node; `DATABASE_REPLICA_DSN=""`.
3. `docker compose up -d backend`.
4. Update Cloudflare origin if the IP changed.

## Post-incident

- Record RPO/RTO actuals in this file.
- Keep ≥ 30 days of base dumps; 14 days of WAL.
- Secrets: restore from password manager, never from repo.

## Related

- Multi-region: `docs/runbooks/multi-region.md`
- Backups UI: admin API `/api/admin/backups`
