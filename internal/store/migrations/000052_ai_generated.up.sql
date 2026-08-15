ALTER TABLE problems ADD COLUMN IF NOT EXISTS ai_generated BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS idx_problems_ai_generated ON problems(ai_generated) WHERE ai_generated = true;
