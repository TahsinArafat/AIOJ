-- Language configs move out of the container filesystem.
--
-- Until now the judge language list lived in lang/*.yaml, and admin#languages
-- edited those files in place (internal/api/handler/admin_languages.go wrote
-- <langDir>/<key>.yaml). That works on a dev box, where docker-compose.yml bind
-- mounts ./lang into every container. It does not work on Dokploy, where the
-- backend has no such mount: the write lands in the container's writable layer,
-- and the next deploy recreates the container from the image and restores the
-- baked-in copy (Dockerfile.api COPY lang ./lang). Every rename and every
-- compiler change an operator made was silently reverted by the next deploy.
--
-- So the live config becomes a row. lang/*.yaml keeps its job as the SHIPPED
-- DEFAULTS — the bytes Reset-to-Default restores and the source the boot-time
-- seeder reads — but it stops being the source of truth.
--
-- One JSONB blob, not a column per field. LangConfig (internal/judge/compiler/
-- lang.go) is already the document shape that both the YAML file and the admin
-- request body use, so storing it verbatim means a new field needs no migration.
-- It is the same choice system_settings made for its values (000033) and the
-- same validation posture contests.access_config documents (000091): the shape
-- is checked in Go, where the handler can answer 400, not by a CHECK that would
-- need rewriting every time a field is added.
--
-- deleted_at, not a row that disappears. Seeding is INSERT-only — it must never
-- overwrite an operator's edit — and that makes a hard DELETE unsafe: the next
-- boot would find the shipped key missing and seed it right back. A soft delete
-- still occupies the primary key, so ON CONFLICT DO NOTHING leaves it alone and
-- the deletion sticks. It also keeps updated_by meaningful for a removed
-- language, which is what an audit question actually asks.
CREATE TABLE IF NOT EXISTS languages (
    key        VARCHAR(64) PRIMARY KEY,
    config     JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    deleted_at TIMESTAMPTZ
);

-- The hot read is "every live language", once per cache fill. A partial index
-- keeps soft-deleted rows out of it and stays small as deleted rows accumulate.
CREATE INDEX IF NOT EXISTS idx_languages_live ON languages(key) WHERE deleted_at IS NULL;
