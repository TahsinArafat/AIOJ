-- Restore original default (problems were visible by default before this migration)
ALTER TABLE problems ALTER COLUMN visible SET DEFAULT true;
