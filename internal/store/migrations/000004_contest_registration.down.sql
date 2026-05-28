DROP TABLE IF EXISTS contest_registrations;
ALTER TABLE contests DROP COLUMN IF EXISTS registration_required;
ALTER TABLE contests DROP COLUMN IF EXISTS registration_deadline;
ALTER TABLE contests DROP COLUMN IF EXISTS max_participants;
