-- Full-text search for problems.
--
-- Ported from dmoj's judge/fulltext.py, which maintains a real full-text index
-- rather than a LIKE scan. AIOJ's search was `title ILIKE '%term%'`, which
-- cannot use a btree index and degrades to a sequential scan on every query.
--
-- A generated column is used instead of a trigger so the search vector can
-- never drift from the row it describes: Postgres recomputes it on write.
--
-- 'simple' rather than 'english': problem text mixes English statements with
-- Bengali and code identifiers, and the english configuration would stem and
-- drop stopwords in ways that mangle both. 'simple' just lowercases and splits
-- on whitespace/punctuation, which also makes substring search predictable.
ALTER TABLE problems
    ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (
        to_tsvector('simple',
            COALESCE(title, '') || ' ' ||
            COALESCE(slug, '') || ' ' ||
            COALESCE(source, '') || ' ' ||
            COALESCE(description, '')
        )
    ) STORED;

-- GIN is the index type for tsvector containment (@@) queries.
CREATE INDEX IF NOT EXISTS idx_problems_search_vector
    ON problems USING GIN (search_vector);

-- Trigram index so the existing substring search (ILIKE '%term%') also stops
-- doing sequential scans. pg_trgm is the standard extension for this.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_problems_title_trgm
    ON problems USING GIN (title gin_trgm_ops);
