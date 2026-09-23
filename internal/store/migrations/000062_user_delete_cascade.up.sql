-- GDPR account deletion: make every FK to users deletable.
-- Personal ownership rows cascade with the user; shared content keeps a
-- null author so community material survives creator deletion.

-- Personal data (hard-delete with user)
ALTER TABLE submissions DROP CONSTRAINT IF EXISTS submissions_user_id_fkey;
ALTER TABLE submissions ADD CONSTRAINT submissions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE contest_submissions DROP CONSTRAINT IF EXISTS contest_submissions_user_id_fkey;
ALTER TABLE contest_submissions ADD CONSTRAINT contest_submissions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE contest_ranks DROP CONSTRAINT IF EXISTS contest_ranks_user_id_fkey;
ALTER TABLE contest_ranks ADD CONSTRAINT contest_ranks_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE virtual_contests DROP CONSTRAINT IF EXISTS virtual_contests_user_id_fkey;
ALTER TABLE virtual_contests ADD CONSTRAINT virtual_contests_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE editorials DROP CONSTRAINT IF EXISTS editorials_user_id_fkey;
ALTER TABLE editorials ADD CONSTRAINT editorials_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE hacks DROP CONSTRAINT IF EXISTS hacks_hacker_id_fkey;
ALTER TABLE hacks ADD CONSTRAINT hacks_hacker_id_fkey
    FOREIGN KEY (hacker_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE hacks DROP CONSTRAINT IF EXISTS hacks_defender_id_fkey;
ALTER TABLE hacks ADD CONSTRAINT hacks_defender_id_fkey
    FOREIGN KEY (defender_id) REFERENCES users(id) ON DELETE CASCADE;

-- Shared content: drop NOT NULL authorship, SET NULL on delete
ALTER TABLE problems ALTER COLUMN created_by DROP NOT NULL;
ALTER TABLE problems DROP CONSTRAINT IF EXISTS problems_created_by_fkey;
ALTER TABLE problems ADD CONSTRAINT problems_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE contests ALTER COLUMN created_by DROP NOT NULL;
ALTER TABLE contests DROP CONSTRAINT IF EXISTS contests_created_by_fkey;
ALTER TABLE contests ADD CONSTRAINT contests_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE groups ALTER COLUMN created_by DROP NOT NULL;
ALTER TABLE groups DROP CONSTRAINT IF EXISTS groups_created_by_fkey;
ALTER TABLE groups ADD CONSTRAINT groups_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE gym_contests ALTER COLUMN created_by DROP NOT NULL;
ALTER TABLE gym_contests DROP CONSTRAINT IF EXISTS gym_contests_created_by_fkey;
ALTER TABLE gym_contests ADD CONSTRAINT gym_contests_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE teams ALTER COLUMN created_by DROP NOT NULL;
ALTER TABLE teams DROP CONSTRAINT IF EXISTS teams_created_by_fkey;
ALTER TABLE teams ADD CONSTRAINT teams_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE contest_notices ALTER COLUMN created_by DROP NOT NULL;
ALTER TABLE contest_notices DROP CONSTRAINT IF EXISTS contest_notices_created_by_fkey;
ALTER TABLE contest_notices ADD CONSTRAINT contest_notices_created_by_fkey
    FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

-- Nullable audit columns
ALTER TABLE system_settings DROP CONSTRAINT IF EXISTS system_settings_updated_by_fkey;
ALTER TABLE system_settings ADD CONSTRAINT system_settings_updated_by_fkey
    FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE onsite_batch_users DROP CONSTRAINT IF EXISTS onsite_batch_users_used_by_fkey;
ALTER TABLE onsite_batch_users ADD CONSTRAINT onsite_batch_users_used_by_fkey
    FOREIGN KEY (used_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE clarifications DROP CONSTRAINT IF EXISTS clarifications_answered_by_fkey;
ALTER TABLE clarifications ADD CONSTRAINT clarifications_answered_by_fkey
    FOREIGN KEY (answered_by) REFERENCES users(id) ON DELETE SET NULL;
