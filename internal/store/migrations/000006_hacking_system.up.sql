CREATE TABLE hacks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id UUID NOT NULL REFERENCES contests(id),
    problem_id UUID NOT NULL REFERENCES problems(id),
    hacker_id UUID NOT NULL REFERENCES users(id),
    defender_id UUID NOT NULL REFERENCES users(id),
    submission_id UUID NOT NULL REFERENCES submissions(id),
    test_input TEXT NOT NULL,
    expected_output TEXT,
    actual_output TEXT,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'success', 'failure', 'error')),
    success BOOLEAN,
    hacker_rating_change INTEGER NOT NULL DEFAULT 0,
    defender_rating_change INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    judged_at TIMESTAMPTZ
);

CREATE INDEX idx_hacks_contest ON hacks(contest_id);
CREATE INDEX idx_hacks_hacker ON hacks(hacker_id);
CREATE INDEX idx_hacks_defender ON hacks(defender_id);
CREATE INDEX idx_hacks_submission ON hacks(submission_id);

ALTER TABLE contests ADD COLUMN IF NOT EXISTS hack_phase_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS hack_phase_start TIMESTAMPTZ;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS hack_phase_end TIMESTAMPTZ;
