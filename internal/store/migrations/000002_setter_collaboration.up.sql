ALTER TABLE problems ADD COLUMN IF NOT EXISTS visible BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS problem_permissions (
    problem_id UUID REFERENCES problems(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    access_level VARCHAR(16) NOT NULL CHECK (access_level IN ('owner', 'co_author', 'tester')),
    PRIMARY KEY (problem_id, user_id)
);

CREATE TABLE IF NOT EXISTS contest_permissions (
    contest_id UUID REFERENCES contests(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    access_level VARCHAR(16) NOT NULL CHECK (access_level IN ('manager', 'tester')),
    PRIMARY KEY (contest_id, user_id)
);

CREATE TABLE IF NOT EXISTS setter_applications (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(16) NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default admin user: username='admin', email='admin@aioj.net', password='admin_secret', role='admin'
INSERT INTO users (id, username, email, password_hash, role, is_bot)
VALUES ('00000000-0000-0000-0000-000000000000', 'admin', 'admin@aioj.net', '$2a$12$CMuYP1U0znkFmeE4E02nTOVTVzPeMLJMoe1fXU23PMjWy5xcDvn2i', 'admin', false)
ON CONFLICT (username) DO NOTHING;

INSERT INTO user_profiles (user_id, rating, problems_solved, submissions, bio)
VALUES ('00000000-0000-0000-0000-000000000000', 1500, 0, 0, 'System Administrator')
ON CONFLICT (user_id) DO NOTHING;
