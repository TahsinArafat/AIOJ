-- Customization and control: profile, teams, groups
-- See docs/superpowers/specs/2026-06-04-customization-and-control-design.md

-- A. User Profile Settings
ALTER TABLE user_profiles
    ADD COLUMN first_name VARCHAR(64) DEFAULT '',
    ADD COLUMN last_name VARCHAR(64) DEFAULT '',
    ADD COLUMN country VARCHAR(64) DEFAULT '',
    ADD COLUMN city VARCHAR(64) DEFAULT '',
    ADD COLUMN organization VARCHAR(128) DEFAULT '',
    ADD COLUMN github_url VARCHAR(256) DEFAULT '',
    ADD COLUMN show_email BOOLEAN DEFAULT FALSE,
    ADD COLUMN show_tags BOOLEAN DEFAULT TRUE;

-- B. Team Settings and Membership States
ALTER TABLE teams ADD COLUMN is_public BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE team_members DROP CONSTRAINT IF EXISTS team_members_role_check;
ALTER TABLE team_members ADD CONSTRAINT team_members_role_check CHECK (role IN ('owner', 'captain', 'member', 'invited', 'requested'));

-- C. Group Settings and Join Policies
ALTER TABLE groups
    ADD COLUMN invite_code VARCHAR(8) UNIQUE,
    ADD COLUMN join_policy VARCHAR(16) NOT NULL DEFAULT 'auto_approve' CHECK (join_policy IN ('auto_approve', 'manual_approve'));

ALTER TABLE group_members DROP CONSTRAINT IF EXISTS group_members_role_check;
ALTER TABLE group_members ADD CONSTRAINT group_members_role_check CHECK (role IN ('owner', 'manager', 'member', 'invited', 'requested'));
