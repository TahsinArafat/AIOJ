-- Companion to 000054: the partial index migration 000038 originally created.
--
-- It lives in its own migration because PostgreSQL will not let a transaction
-- reference an enum value added in that same transaction, and golang-migrate
-- wraps every migration in one. By the time this runs, 'rejudging' is committed
-- in 000054, so the index predicate below is legal.
--
-- IF NOT EXISTS keeps this safe on databases where 000038's index already landed.
DROP INDEX IF EXISTS idx_submissions_pending_poll;
CREATE INDEX idx_submissions_pending_poll
    ON submissions(status, remote_id)
    WHERE remote_id != '' AND status IN ('pending', 'rejudging');
