-- Editorial metadata fields.
--
-- internal/store/postgres/editorials.go inserts and selects
-- approach / time_complexity / space_complexity (see the Create and GetByID
-- statements, and model.Editorial's matching fields), but no migration ever
-- created the columns. Every editorial INSERT therefore failed with
-- "column ... does not exist" and every editorial SELECT with the same.
-- Found by auditing the store layer's column references against the live schema.
ALTER TABLE editorials ADD COLUMN IF NOT EXISTS approach TEXT NOT NULL DEFAULT '';
ALTER TABLE editorials ADD COLUMN IF NOT EXISTS time_complexity VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE editorials ADD COLUMN IF NOT EXISTS space_complexity VARCHAR(64) NOT NULL DEFAULT '';
