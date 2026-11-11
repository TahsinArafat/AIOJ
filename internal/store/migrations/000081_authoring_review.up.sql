-- Publishing was previously a plain boolean, so any setter could put a problem in
-- the public archive the moment they ticked "visible" — on create, on edit, or by
-- using any of the five importers, which hardcoded visible = true. There was no
-- review and no record of who approved what.
--
-- status records the workflow; visible stays the derived "is it readable by the
-- public" flag that every existing query already filters on, so the archive,
-- search, feed and sitemap needed no changes.
--
--   draft      never submitted, or sent back for changes
--   pending    awaiting an admin
--   published  approved; visible is forced true
--   rejected   refused; visible is forced false

ALTER TABLE problems
    ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS review_note TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;

-- Backfill before adding the constraint: every already-public problem is
-- published, everything else is a draft. Without this, the CHECK below would
-- reject the "draft" default for the rows that are already visible.
UPDATE problems SET status = CASE WHEN visible THEN 'published' ELSE 'draft' END;

ALTER TABLE problems
    ADD CONSTRAINT problems_status_check
    CHECK (status IN ('draft', 'pending', 'published', 'rejected'));

-- Keep the boolean and the status from disagreeing. Existing readers filter on
-- visible, so the invariant is what lets them stay unchanged.
ALTER TABLE problems
    ADD CONSTRAINT problems_published_visible
    CHECK (status <> 'published' OR visible);

-- The admin review queue lists pending problems oldest-first; a partial index
-- keeps that off the full table.
CREATE INDEX IF NOT EXISTS idx_problems_pending_review
    ON problems (created_at)
    WHERE status = 'pending';

-- Contests get the same treatment, and is_rated finally becomes readable. It has
-- existed since migration 000003 and no Go code ever selected or wrote it, so
-- every contest was implicitly rated and an unrated one could not be created.
ALTER TABLE contests
    ADD COLUMN IF NOT EXISTS status VARCHAR(16) NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS review_note TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;

UPDATE contests SET status = CASE WHEN visible THEN 'published' ELSE 'draft' END;

ALTER TABLE contests
    ADD CONSTRAINT contests_status_check
    CHECK (status IN ('draft', 'pending', 'published', 'rejected'));

ALTER TABLE contests
    ADD CONSTRAINT contests_published_visible
    CHECK (status <> 'published' OR visible);

CREATE INDEX IF NOT EXISTS idx_contests_pending_review
    ON contests (created_at)
    WHERE status = 'pending';
