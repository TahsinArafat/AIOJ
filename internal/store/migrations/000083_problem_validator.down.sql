ALTER TABLE problem_generation_assets
    DROP COLUMN IF EXISTS validator_source,
    DROP COLUMN IF EXISTS validator_language;

ALTER TABLE problems
    DROP COLUMN IF EXISTS validator_source,
    DROP COLUMN IF EXISTS validator_language;
