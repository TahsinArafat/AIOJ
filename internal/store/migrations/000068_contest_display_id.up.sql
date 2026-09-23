-- Contest display numbers (Codeforces-style round ids).
--
-- The application already exposes display_id and resolves a bare numeric
-- contest URL through it, but the column was missing from the schema. Allocate
-- stable positive values for existing rows and make the database assign the
-- next value for every new contest.
CREATE SEQUENCE IF NOT EXISTS contests_display_id_seq;

ALTER TABLE contests ADD COLUMN IF NOT EXISTS display_id INTEGER;

WITH numbered AS (
    SELECT id, ROW_NUMBER() OVER (ORDER BY created_at, id) AS next_id
    FROM contests
    WHERE display_id IS NULL OR display_id = 0
)
UPDATE contests c
   SET display_id = numbered.next_id
  FROM numbered
 WHERE c.id = numbered.id;

SELECT setval(
    'contests_display_id_seq',
    GREATEST(COALESCE((SELECT MAX(display_id) FROM contests), 0), 1),
    COALESCE((SELECT MAX(display_id) FROM contests), 0) > 0
);

ALTER TABLE contests
    ALTER COLUMN display_id SET DEFAULT nextval('contests_display_id_seq'),
    ALTER COLUMN display_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_contests_display_id
    ON contests(display_id);
