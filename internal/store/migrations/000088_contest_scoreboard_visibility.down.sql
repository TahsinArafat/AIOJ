ALTER TABLE contests DROP CONSTRAINT IF EXISTS contests_scoreboard_visibility_check;
ALTER TABLE contests DROP COLUMN IF EXISTS scoreboard_visibility;
