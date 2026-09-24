-- The backfill only inserted default profile rows, which are indistinguishable
-- from rows a user would have had created lazily. Rolling back would therefore
-- delete real (if empty) profiles for users who legitimately have one, so there
-- is nothing safe to undo.
SELECT 1;
