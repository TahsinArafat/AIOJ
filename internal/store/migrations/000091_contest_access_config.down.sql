-- Reconstruct the column, then the settings.
--
-- The reverse of the up migration, and it has to run in that order: the column
-- is added with its default before any row is read from the map, so a round with
-- no standings entry lands on 'participants' rather than on NULL against a
-- NOT NULL column.
--
-- Only 'standings' can be reconstructed. The other five surfaces have no column
-- to go back to and their settings are lost, which is inherent in a down
-- migration from a map to a scalar — it is recorded here rather than hidden
-- behind a best-effort guess, because an audience silently changing its meaning
-- on a rollback is worse than a setting reverting to the default.
ALTER TABLE contests
    ADD COLUMN IF NOT EXISTS scoreboard_visibility VARCHAR(16) NOT NULL DEFAULT 'participants';

UPDATE contests
   SET scoreboard_visibility = 'public'
 WHERE access_config->>'standings' = 'public';

ALTER TABLE contests
    ADD CONSTRAINT contests_scoreboard_visibility_check
    CHECK (scoreboard_visibility IN ('participants', 'public'));

ALTER TABLE contests DROP COLUMN IF EXISTS access_config;
