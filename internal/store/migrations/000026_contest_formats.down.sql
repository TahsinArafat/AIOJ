-- Remove index
DROP INDEX IF EXISTS idx_contests_format;

-- Remove columns
ALTER TABLE contests DROP COLUMN IF EXISTS format_config;
ALTER TABLE contests DROP COLUMN IF EXISTS format;
