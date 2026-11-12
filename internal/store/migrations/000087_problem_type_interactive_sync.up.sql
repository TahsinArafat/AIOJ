-- Keep problems.interactive and problems.problem_type from disagreeing.
--
-- Migration 000082 introduced problem_type and backfilled it from the older
-- `interactive` boolean exactly once:
--
--     UPDATE problems SET problem_type = 'interactive' WHERE interactive = true;
--
-- Nothing maintained that afterwards, and nothing tied the two together. The
-- setter workspace toggles `interactive` (CheckerTab), the handler wrote
-- p.Interactive, and p.ProblemType was never assigned — so an author ticking
-- "Interactive" produced a row that IS interactive for the judge (worker.go keys
-- off the boolean) but reports problem_type = 'standard' to the submission path
-- (submission.go branches on 'output_only') and to the public badge.
--
-- The handlers now resolve both fields together (model.ResolveProblemType), but
-- a constraint is what stops a future caller from reintroducing the gap.
--
-- The equivalence is written as "interactive implies type = interactive" rather
-- than equality because output_only is a distinct type with interactive = false,
-- and standard is the other non-interactive case. That is exactly the invariant:
-- the boolean may only be true when the type says interactive.
--
-- The reverse (type = 'interactive' implies the boolean is true) is enforced by
-- the handlers rather than here: an interactive problem needs an interactor to
-- run, so a row claiming the type without the boolean is the same defect from
-- the other side, and rejecting it in one direction is enough to make the user
-- fix the pair.

-- Repair rows the handlers left divergent before this constraint existed: a
-- problem whose type is interactive but whose boolean is off cannot be judged
-- interactively, and a problem whose boolean is on but whose type is standard
-- would lose its interactor on any path that reads the type.
UPDATE problems SET interactive = true  WHERE problem_type = 'interactive' AND interactive = false;
UPDATE problems SET problem_type = 'interactive' WHERE interactive = true AND problem_type <> 'interactive';

ALTER TABLE problems DROP CONSTRAINT IF EXISTS problems_interactive_type_check;
ALTER TABLE problems
    ADD CONSTRAINT problems_interactive_type_check
    CHECK (NOT interactive OR problem_type = 'interactive');

COMMENT ON CONSTRAINT problems_interactive_type_check ON problems IS
    'interactive may only be true when problem_type is interactive; the two fields describe the same problem (migration 000087)';
