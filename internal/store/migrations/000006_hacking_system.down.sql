DROP TABLE IF EXISTS hacks;
ALTER TABLE contests DROP COLUMN IF EXISTS hack_phase_enabled;
ALTER TABLE contests DROP COLUMN IF EXISTS hack_phase_start;
ALTER TABLE contests DROP COLUMN IF EXISTS hack_phase_end;
