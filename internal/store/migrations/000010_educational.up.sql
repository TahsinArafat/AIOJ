ALTER TABLE contests ADD COLUMN IF NOT EXISTS educational_config JSONB;

CREATE TABLE IF NOT EXISTS editorials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    contest_id UUID REFERENCES contests(id),
    user_id UUID NOT NULL REFERENCES users(id),
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL,
    solution_code TEXT,
    solution_language VARCHAR(64),
    is_official BOOLEAN NOT NULL DEFAULT false,
    upvotes INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_editorials_problem ON editorials(problem_id);
CREATE INDEX idx_editorials_contest ON editorials(contest_id) WHERE contest_id IS NOT NULL;
