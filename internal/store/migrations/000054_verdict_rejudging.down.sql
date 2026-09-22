-- Postgres cannot remove a value from an enum, so the down migration only
-- drops the index that migration 38 introduced and that this migration
-- recreated. The 'rejudging' label stays; that is intentional and matches
-- the Go constant in internal/model/submission.go.
DROP INDEX IF EXISTS idx_submissions_pending_poll;
