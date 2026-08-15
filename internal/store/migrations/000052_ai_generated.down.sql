DROP INDEX IF EXISTS idx_problems_ai_generated;
ALTER TABLE problems DROP COLUMN IF EXISTS ai_generated;
