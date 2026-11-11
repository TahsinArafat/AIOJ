DROP INDEX IF EXISTS idx_contests_pending_review;
DROP INDEX IF EXISTS idx_problems_pending_review;

ALTER TABLE contests
    DROP CONSTRAINT IF EXISTS contests_published_visible,
    DROP CONSTRAINT IF EXISTS contests_status_check;

ALTER TABLE contests
    DROP COLUMN IF EXISTS reviewed_at,
    DROP COLUMN IF EXISTS reviewed_by,
    DROP COLUMN IF EXISTS review_note,
    DROP COLUMN IF EXISTS status;

ALTER TABLE problems
    DROP CONSTRAINT IF EXISTS problems_published_visible,
    DROP CONSTRAINT IF EXISTS problems_status_check;

ALTER TABLE problems
    DROP COLUMN IF EXISTS reviewed_at,
    DROP COLUMN IF EXISTS reviewed_by,
    DROP COLUMN IF EXISTS review_note,
    DROP COLUMN IF EXISTS status;
