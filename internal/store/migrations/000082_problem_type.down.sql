DROP INDEX IF EXISTS idx_problems_problem_type;

ALTER TABLE problems
    DROP CONSTRAINT IF EXISTS problems_problem_type_check;

ALTER TABLE problems
    DROP COLUMN IF EXISTS problem_type;
