DROP INDEX IF EXISTS idx_contests_display_id;
ALTER TABLE contests ALTER COLUMN display_id DROP DEFAULT;
DROP SEQUENCE IF EXISTS contests_display_id_seq;
ALTER TABLE contests DROP COLUMN IF EXISTS display_id;
