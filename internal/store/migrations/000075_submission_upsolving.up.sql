-- Add is_upsolving flag to submissions so rejudge and analytics can correctly
-- identify post-contest upsolve submissions without re-fetching contest times.
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS is_upsolving BOOLEAN NOT NULL DEFAULT FALSE;

-- Index so we can query upsolving vs contest submissions efficiently.
CREATE INDEX IF NOT EXISTS idx_submissions_is_upsolving
    ON submissions (contest_id, is_upsolving)
    WHERE contest_id IS NOT NULL;
