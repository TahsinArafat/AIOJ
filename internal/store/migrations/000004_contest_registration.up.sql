CREATE TABLE contest_registrations (
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contest_id, user_id)
);

CREATE INDEX idx_contest_registrations_user ON contest_registrations(user_id);

ALTER TABLE contests ADD COLUMN IF NOT EXISTS registration_required BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS registration_deadline TIMESTAMPTZ;
ALTER TABLE contests ADD COLUMN IF NOT EXISTS max_participants INTEGER;
