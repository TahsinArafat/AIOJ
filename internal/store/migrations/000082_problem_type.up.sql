-- Output-only problems were supported by the judge all along: OutputOnlyRunner
-- (internal/judge/runner/interactive.go:147) treats the submission's source as
-- the contestant's output and is dispatched at internal/judge/worker.go:242
-- before any language lookup. submissions.submission_type has existed since
-- migration 000024. But nothing could ever CREATE such a submission, and there
-- was nowhere to mark a problem as output-only.
--
-- problem_type is that marker. 'interactive' is backfilled from the boolean that
-- already existed so the two agree; the submission path keys off this column to
-- decide whether to accept a code submission or an output file.
ALTER TABLE problems
    ADD COLUMN IF NOT EXISTS problem_type VARCHAR(16) NOT NULL DEFAULT 'standard';

UPDATE problems SET problem_type = 'interactive' WHERE interactive = true;

ALTER TABLE problems
    DROP CONSTRAINT IF EXISTS problems_problem_type_check;
ALTER TABLE problems
    ADD CONSTRAINT problems_problem_type_check
    CHECK (problem_type IN ('standard', 'interactive', 'output_only'));

CREATE INDEX IF NOT EXISTS idx_problems_problem_type
    ON problems (problem_type) WHERE problem_type <> 'standard';
