-- Verdict self-heal: track when a submission entered the 'judging' state so a
-- crashed / restarted judge-worker's orphaned claims can be reclaimed and
-- re-enqueued instead of being stuck forever.
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS judging_started_at TIMESTAMPTZ;

-- Partial index keeps the reclaim sweep cheap: only rows actually being judged
-- are indexed, which is a tiny fraction of the table.
CREATE INDEX IF NOT EXISTS idx_submissions_judging_stale
    ON submissions (judging_started_at)
    WHERE status = 'judging';
