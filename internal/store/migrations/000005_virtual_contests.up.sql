CREATE TABLE virtual_contests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    original_contest_id UUID NOT NULL REFERENCES contests(id),
    user_id UUID NOT NULL REFERENCES users(id),
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    duration_minutes INTEGER NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'abandoned')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_virtual_contests_user ON virtual_contests(user_id);
CREATE INDEX idx_virtual_contests_original ON virtual_contests(original_contest_id);
CREATE INDEX idx_virtual_contests_status ON virtual_contests(status) WHERE status = 'active';
