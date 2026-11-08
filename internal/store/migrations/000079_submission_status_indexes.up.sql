-- Indexes for the site-wide status page and the per-problem setter listing.
--
-- Both list submissions newest-first. Until now the only created_at indexes
-- were (user_id, created_at) and (contest_id, created_at), so an unfiltered
-- "latest submissions" page had to sort the whole table for every request and
-- a problem's submission list had to sort that problem's rows each time.
CREATE INDEX IF NOT EXISTS idx_submissions_created_at
    ON submissions (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_submissions_problem_created
    ON submissions (problem_id, created_at DESC);
