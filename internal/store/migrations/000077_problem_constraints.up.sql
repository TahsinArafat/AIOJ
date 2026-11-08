-- Dedicated constraints block, rendered as its own section on the problem page.
--
-- Constraints previously lived only inside the free-form description, so a
-- problem could ship with no bounds at all and nothing on the page marked them
-- as constraints. The harness already collects the numeric bounds structurally
-- (set_constraints) and renders them here as a LaTeX block, which the problem
-- page shows between Output format and Notes.
ALTER TABLE problems ADD COLUMN IF NOT EXISTS constraints TEXT NOT NULL DEFAULT '';
