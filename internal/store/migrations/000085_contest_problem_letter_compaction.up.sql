-- Compact every contest's problem letters so they are a gapless A, B, C… again.
--
-- A problem's letter was stored in contest_problems.index and never revisited
-- when a problem was removed, so a deletion left a permanent hole. Worse, the
-- stored letter was then only half-believed: the problems tab read it, while the
-- standings header and the PDF booklet each computed `'A' + position` for
-- themselves. A contest that lost its first problem therefore showed "B" on one
-- tab and "A" in the standings for the same problem.
--
-- The source of truth is now position in sort_order (model.ContestProblemIndex),
-- and this migration brings the stored data in line, so a contest that already
-- has a gap is repaired now rather than waiting for its next edit.
--
-- Ordering matches the Go renumber exactly: sort_order first, index as the
-- tie-break, so a round whose sort_order was never maintained still comes out in
-- the order a reader expects.
--
-- 26 is the ceiling: contest_problems.index is CHAR(1). A contest with more than
-- 26 problems cannot be represented and is left untouched rather than silently
-- truncated or written as an invalid letter.
WITH ordered AS (
    SELECT contest_id,
           problem_id,
           ROW_NUMBER() OVER (
               PARTITION BY contest_id
               ORDER BY sort_order, index
           ) - 1 AS position,
           COUNT(*) OVER (PARTITION BY contest_id) AS total
      FROM contest_problems
),
renumbered AS (
    SELECT contest_id,
           problem_id,
           CHR((65 + position)::int) AS new_index,
           position           AS new_sort_order
      FROM ordered
     WHERE total <= 26
)
UPDATE contest_problems cp
   SET index      = r.new_index,
       sort_order = r.new_sort_order
  FROM renumbered r
 WHERE cp.contest_id = r.contest_id
   AND cp.problem_id = r.problem_id
   AND (cp.index <> r.new_index OR cp.sort_order <> r.new_sort_order);
