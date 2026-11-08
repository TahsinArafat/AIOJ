-- A System Error means the judge failed, not the contestant. Until now the
-- only trace of *why* was the worker's stdout log and the error string stuffed
-- into compile_output, which the UI hides unless the verdict is a compile
-- error — so a submitter reported "system error" and nobody could see the
-- cause from the app at all.
--
-- judge_error keeps that failure text as its own field: readable by admins and
-- the problem's setters, never rendered as if the contestant's code failed to
-- compile. Existing rows are backfilled from compile_output for the same
-- reason (an SE never has real compiler output).
ALTER TABLE submissions
    ADD COLUMN IF NOT EXISTS judge_error TEXT NOT NULL DEFAULT '';

UPDATE submissions
   SET judge_error = compile_output
 WHERE status = 'se'
   AND judge_error = ''
   AND compile_output <> '';

-- The admin "judge failures" panel lists newest-first.
CREATE INDEX IF NOT EXISTS idx_submissions_se_created
    ON submissions (created_at DESC)
    WHERE status = 'se';
