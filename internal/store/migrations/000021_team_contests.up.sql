-- Add team contest support
ALTER TABLE contests ADD COLUMN IF NOT EXISTS team_size INTEGER DEFAULT 1;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS is_team_contest BOOLEAN DEFAULT FALSE;

-- Team registrations table
CREATE TABLE IF NOT EXISTS team_registrations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    registered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(contest_id, team_id)
);
