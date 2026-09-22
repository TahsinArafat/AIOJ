DROP INDEX IF EXISTS idx_problems_title_trgm;
DROP INDEX IF EXISTS idx_problems_search_vector;
ALTER TABLE problems DROP COLUMN IF EXISTS search_vector;
-- pg_trgm is intentionally left installed: other objects may depend on it, and
-- dropping an extension is not a safe implicit side effect of rolling back a
-- search feature.
