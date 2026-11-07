ALTER TABLE ai_generation_jobs
    DROP COLUMN IF EXISTS harness_version,
    DROP COLUMN IF EXISTS turn_count,
    DROP COLUMN IF EXISTS case_manifest,
    DROP COLUMN IF EXISTS budget_state;

DROP INDEX IF EXISTS idx_ai_harness_turns_job;
DROP TABLE IF EXISTS ai_harness_turns;
