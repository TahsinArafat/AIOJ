DROP INDEX IF EXISTS idx_submissions_judging_stale;
DROP INDEX IF EXISTS idx_submissions_judging_stale;
ALTER TABLE submissions DROP COLUMN IF EXISTS judging_started_at;
