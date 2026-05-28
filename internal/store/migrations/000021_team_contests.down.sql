-- Remove team contest support
DROP TABLE IF EXISTS team_registrations;
ALTER TABLE contests DROP COLUMN IF EXISTS team_size;
ALTER TABLE contests DROP COLUMN IF EXISTS is_team_contest;
