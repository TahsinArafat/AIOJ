-- Keep problems.interactive and problems.problem_type from disagreeing.
--
-- Migration 000082 introduced problem_type and backfilled it from the older
-- `interactive` boolean exactly once:
--
--     UPDATE problems SET problem_type = 'interactive' WHERE interactive = true;
--
-- Nothing maintained that afterwards. The setter workspace toggles `interactive`
-- (CheckerTab) and the handler wrote p.Interactive, but p.ProblemType was never
-- assigned — so an author ticking "Interactive" produced a row that IS
-- interactive for the judge (worker.go keys off the boolean) while reporting
-- problem_type = 'standard' to the submission path and the public badge.
--
-- The handlers now resolve both fields together (model.ResolveProblemType). This
-- migration repairs rows that already diverged and adds the constraint that
-- stops a future caller from reopening the gap.
--
-- IMPORTANT — every repair below is chosen to preserve what production actually
-- does today. The judge dispatches on `interactive` (worker.go), and the
-- submission path derives output-only from problem_type (submission.go). Writing
-- either field the "obvious" way would silently change how existing problems are
-- graded:
--
--   * Flipping interactive=false to true for an 'interactive' row would send it
--     down judgeInteractiveSub, which compiles problems.interactor_source_code.
--     A row that never had an interactor fails to compile and every submission
--     turns SE. So those rows are demoted to 'standard' instead, which is what
--     the judge already did with them.
--
--   * Flipping an output_only row to 'interactive' would change how its
--     submissions are accepted. Those keep their type and lose the contradictory
--     boolean instead, matching the output-only path that already short-circuits
--     before the interactive branch.
--
-- Run the preflight below against production before deploying to see exactly
-- which rows will change:
--
--   SELECT id, slug, interactive, problem_type
--     FROM problems
--    WHERE (interactive AND problem_type <> 'interactive')
--       OR (NOT interactive AND problem_type = 'interactive');

-- 1. Contradictory pair: interactive bool on, type says output_only. The judge
--    already grades these as output-only (submission type short-circuits first),
--    so keep the type and clear the flag.
UPDATE problems
   SET interactive = false
 WHERE interactive = true
   AND problem_type = 'output_only';

-- 2. interactive bool on, type standard: promote the type. The judge already
--    treats these as interactive and already requires an interactor to run them,
--    so this changes no behavior — it only makes the enum agree.
UPDATE problems
   SET problem_type = 'interactive'
 WHERE interactive = true
   AND problem_type = 'standard';

-- 3. Type says interactive but the boolean is off: the judge has been grading
--    these as ordinary code submissions. Demote the type so they keep doing
--    exactly that, rather than being flipped into interactive judging.
UPDATE problems
   SET problem_type = 'standard'
 WHERE problem_type = 'interactive'
   AND interactive = false;

-- The invariant, written one way only: the boolean may be true only when the
-- type says interactive. output_only and standard are both valid non-interactive
-- types, so this is not an equality between the two columns.
ALTER TABLE problems DROP CONSTRAINT IF EXISTS problems_interactive_type_check;
ALTER TABLE problems
    ADD CONSTRAINT problems_interactive_type_check
    CHECK (NOT interactive OR problem_type = 'interactive');

COMMENT ON CONSTRAINT problems_interactive_type_check ON problems IS
    'interactive may only be true when problem_type is interactive; the two fields describe the same problem (migration 000089)';
