-- Restore original access_level constraint (without 'judge')
ALTER TABLE contest_permissions DROP CONSTRAINT IF EXISTS contest_permissions_access_level_check;
ALTER TABLE contest_permissions ADD CONSTRAINT contest_permissions_access_level_check
    CHECK (access_level IN ('manager', 'tester'));

-- Remove columns added in up
ALTER TABLE contests DROP COLUMN IF EXISTS upsolving_enabled;
ALTER TABLE contests DROP COLUMN IF EXISTS virtual_contest_enabled;

-- Drop onsite_batch_users table
DROP INDEX IF EXISTS idx_onsite_batch_users_contest;
DROP INDEX IF EXISTS idx_onsite_batch_users_username;
DROP TABLE IF EXISTS onsite_batch_users;
