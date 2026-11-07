DROP INDEX IF EXISTS idx_submissions_is_upsolving;
ALTER TABLE submissions DROP COLUMN IF EXISTS is_upsolving;
