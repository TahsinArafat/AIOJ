CREATE TABLE gym_contests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    difficulty_rating INTEGER CHECK (difficulty_rating >= 800 AND difficulty_rating <= 3500),
    category VARCHAR(64) NOT NULL DEFAULT 'general',
    country VARCHAR(64),
    season VARCHAR(32),
    description TEXT NOT NULL DEFAULT '',
    is_public BOOLEAN NOT NULL DEFAULT true,
    solve_count INTEGER NOT NULL DEFAULT 0,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE gym_solves (
    gym_id UUID NOT NULL REFERENCES gym_contests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    solved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (gym_id, user_id)
);

CREATE INDEX idx_gym_category ON gym_contests(category);
CREATE INDEX idx_gym_difficulty ON gym_contests(difficulty_rating);
CREATE INDEX idx_gym_public ON gym_contests(is_public) WHERE is_public = true;
