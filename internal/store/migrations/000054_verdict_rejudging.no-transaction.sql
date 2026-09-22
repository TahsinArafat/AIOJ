-- 'rejudging' is a first-class submission status in the Go layer
-- (internal/model/submission.go: StatusRejudging SubmissionStatus = "rejudging"),
-- but it was never added to the `verdict` enum. That left the constant
-- unwritable against submissions.status (type verdict) and made migration
-- 000038_multi_bot fail on any database applying it for the first time:
--
--   ERROR: invalid input value for enum verdict: "rejudging"
--
-- migration 38's partial index is recreated here once the value exists.
-- IF NOT EXISTS keeps this safe on databases where the index already landed.
ALTER TYPE verdict ADD VALUE IF NOT EXISTS 'rejudging';

CREATE INDEX IF NOT EXISTS idx_submissions_pending_poll
    ON submissions(status, remote_id)
    WHERE remote_id != '' AND status IN ('pending', 'rejudging');
