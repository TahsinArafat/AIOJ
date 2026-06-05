CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(64) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(16) NOT NULL DEFAULT 'user' CHECK (role IN ('admin','setter','user','bot')),
    is_bot BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_profiles (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    rating INTEGER NOT NULL DEFAULT 0,
    problems_solved INTEGER NOT NULL DEFAULT 0,
    submissions INTEGER NOT NULL DEFAULT 0,
    bio TEXT,
    avatar_url VARCHAR(512)
);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);

CREATE TABLE problems (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(128) UNIQUE NOT NULL,
    title VARCHAR(256) NOT NULL,
    description TEXT NOT NULL,
    input_format TEXT NOT NULL DEFAULT '',
    output_format TEXT NOT NULL DEFAULT '',
    hint TEXT NOT NULL DEFAULT '',
    sample_cases JSONB NOT NULL DEFAULT '[]',
    time_limit INTEGER NOT NULL DEFAULT 1000,
    memory_limit INTEGER NOT NULL DEFAULT 262144,
    difficulty VARCHAR(16) NOT NULL DEFAULT 'easy' CHECK (difficulty IN ('easy','medium','hard')),
    tags TEXT[] NOT NULL DEFAULT '{}',
    visible BOOLEAN NOT NULL DEFAULT false,
    testdata_path VARCHAR(512) NOT NULL DEFAULT '',
    testcase_score JSONB NOT NULL DEFAULT '[]',
    spj BOOLEAN NOT NULL DEFAULT false,
    spj_language VARCHAR(64) NOT NULL DEFAULT '',
    spj_source_code TEXT NOT NULL DEFAULT '',
    spj_version VARCHAR(64) NOT NULL DEFAULT '',
    submission_count INTEGER NOT NULL DEFAULT 0,
    accepted_count INTEGER NOT NULL DEFAULT 0,
    source VARCHAR(64) NOT NULL DEFAULT 'local',
    remote_id VARCHAR(128) NOT NULL DEFAULT '',
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_problems_visible ON problems(visible) WHERE visible = true;
CREATE INDEX idx_problems_tags ON problems USING GIN(tags);

CREATE TYPE verdict AS ENUM ('pending','judging','ac','wa','tle','mle','re','ce','se');

CREATE TABLE contests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(256) NOT NULL,
    type VARCHAR(16) NOT NULL DEFAULT 'acm' CHECK (type IN ('acm','oi','ioi','practice')),
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    freeze_time TIMESTAMPTZ,
    password VARCHAR(128),
    visible BOOLEAN NOT NULL DEFAULT true,
    description TEXT NOT NULL DEFAULT '',
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    problem_id UUID NOT NULL REFERENCES problems(id),
    user_id UUID NOT NULL REFERENCES users(id),
    contest_id UUID REFERENCES contests(id),
    language VARCHAR(64) NOT NULL,
    source_code TEXT NOT NULL,
    code_size INTEGER NOT NULL DEFAULT 0,
    status verdict NOT NULL DEFAULT 'pending',
    score INTEGER NOT NULL DEFAULT 0,
    time_used INTEGER NOT NULL DEFAULT 0,
    memory_used INTEGER NOT NULL DEFAULT 0,
    compile_output TEXT NOT NULL DEFAULT '',
    judge_result JSONB NOT NULL DEFAULT '[]',
    judged_by VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    judged_at TIMESTAMPTZ
);
CREATE INDEX idx_submissions_problem_status ON submissions(problem_id, status);
CREATE INDEX idx_submissions_user_time ON submissions(user_id, created_at DESC);

CREATE TABLE contest_problems (
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    problem_id UUID NOT NULL REFERENCES problems(id),
    index CHAR(1) NOT NULL,
    score INTEGER NOT NULL DEFAULT 100,
    sort_order INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (contest_id, problem_id)
);

CREATE TABLE contest_submissions (
    submission_id UUID PRIMARY KEY REFERENCES submissions(id) ON DELETE CASCADE,
    contest_id UUID NOT NULL REFERENCES contests(id),
    problem_index CHAR(1) NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id)
);

CREATE TABLE contest_ranks (
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id),
    problems JSONB NOT NULL DEFAULT '{}',
    total_solved INTEGER NOT NULL DEFAULT 0,
    total_penalty INTEGER NOT NULL DEFAULT 0,
    total_score INTEGER NOT NULL DEFAULT 0,
    last_ac_time TIMESTAMPTZ,
    PRIMARY KEY (contest_id, user_id)
);

CREATE TABLE contest_announcements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    title VARCHAR(256) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE bot_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(64) NOT NULL,
    platform_user VARCHAR(256) NOT NULL DEFAULT '',
    platform_pass VARCHAR(512) NOT NULL DEFAULT '',
    session_data JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active','expired','error','banned')),
    rate_limit_rps REAL NOT NULL DEFAULT 1.0,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, platform)
);

CREATE INDEX idx_submissions_user ON submissions(user_id);
CREATE INDEX idx_contest_problems_contest ON contest_problems(contest_id);
CREATE INDEX idx_contest_submissions_contest_user ON contest_submissions(contest_id, user_id);
