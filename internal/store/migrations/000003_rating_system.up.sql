-- Rating history table
CREATE TABLE rating_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    old_rating INTEGER NOT NULL,
    new_rating INTEGER NOT NULL,
    rank INTEGER NOT NULL,
    rating_change INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rating_history_user ON rating_history(user_id);
CREATE INDEX idx_rating_history_contest ON rating_history(contest_id);

-- Add rating columns to user_profiles if not exists
ALTER TABLE user_profiles ADD COLUMN IF NOT EXISTS max_rating INTEGER NOT NULL DEFAULT 0;
ALTER TABLE user_profiles ADD COLUMN IF NOT EXISTS contest_count INTEGER NOT NULL DEFAULT 0;

-- Add division columns to contests
ALTER TABLE contests ADD COLUMN IF NOT EXISTS division INTEGER NOT NULL DEFAULT 0;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS rated_for_min INTEGER NOT NULL DEFAULT 0;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS rated_for_max INTEGER NOT NULL DEFAULT 9999;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS is_rated BOOLEAN NOT NULL DEFAULT true;
