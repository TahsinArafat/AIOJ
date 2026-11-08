DROP INDEX IF EXISTS idx_submissions_se_created;
ALTER TABLE submissions DROP COLUMN IF EXISTS judge_error;
