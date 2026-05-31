-- Add 'judge' to contest_permissions access_level
ALTER TABLE contest_permissions DROP CONSTRAINT IF EXISTS contest_permissions_access_level_check;
ALTER TABLE contest_permissions ADD CONSTRAINT contest_permissions_access_level_check
    CHECK (access_level IN ('manager', 'judge', 'tester'));

-- Add upsolving/virtual control columns to contests
ALTER TABLE contests ADD COLUMN IF NOT EXISTS upsolving_enabled BOOLEAN DEFAULT true;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS virtual_contest_enabled BOOLEAN DEFAULT true;

-- Create onsite_batch_users table for temporary credentials
CREATE TABLE IF NOT EXISTS onsite_batch_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    team_name VARCHAR(255) NOT NULL,
    institution VARCHAR(255),
    username VARCHAR(64) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    is_used BOOLEAN DEFAULT false,
    used_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_onsite_batch_users_contest ON onsite_batch_users(contest_id);
CREATE INDEX idx_onsite_batch_users_username ON onsite_batch_users(username);
