-- Irreversible by design.
--
-- The up migration cannot tell a repaired row from one that was always correct,
-- so a down migration would corrupt data rather than restore it. Reintroducing
-- byte values would also re-break the judge and the problem page.
SELECT 1;
