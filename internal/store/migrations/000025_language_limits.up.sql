CREATE TABLE IF NOT EXISTS language_limits (
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    language_id VARCHAR(64) NOT NULL,
    time_limit_ms INTEGER,
    memory_limit_kb INTEGER,
    PRIMARY KEY (problem_id, language_id)
);

CREATE INDEX IF NOT EXISTS idx_language_limits_problem ON language_limits(problem_id);
