-- Rollback customization and control migration

-- A. User Profile Settings
ALTER TABLE user_profiles
    DROP COLUMN IF EXISTS first_name,
    DROP COLUMN IF EXISTS last_name,
    DROP COLUMN IF EXISTS country,
    DROP COLUMN IF EXISTS city,
    DROP COLUMN IF EXISTS organization,
    DROP COLUMN IF EXISTS github_url,
    DROP COLUMN IF EXISTS show_email,
    DROP COLUMN IF EXISTS show_tags;

-- B. Team Settings and Membership States
ALTER TABLE teams DROP COLUMN IF EXISTS is_public;

ALTER TABLE team_members DROP CONSTRAINT IF EXISTS team_members_role_check;
ALTER TABLE team_members ADD CONSTRAINT team_members_role_check CHECK (role IN ('owner', 'captain', 'member'));

-- C. Group Settings and Join Policies
ALTER TABLE groups
    DROP COLUMN IF EXISTS invite_code,
    DROP COLUMN IF EXISTS join_policy;

ALTER TABLE group_members DROP CONSTRAINT IF EXISTS group_members_role_check;
ALTER TABLE group_members ADD CONSTRAINT group_members_role_check CHECK (role IN ('owner', 'admin', 'member'));
