-- Phase D: user achievements / badges
CREATE TABLE IF NOT EXISTS achievements (
    id          UUID PRIMARY KEY,
    code        VARCHAR(64) NOT NULL UNIQUE,
    title       VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    icon        VARCHAR(64) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_achievements (
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_id UUID NOT NULL REFERENCES achievements(id) ON DELETE CASCADE,
    awarded_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, achievement_id)
);

CREATE INDEX IF NOT EXISTS idx_user_achievements_user ON user_achievements(user_id);

-- Seed catalog (idempotent)
INSERT INTO achievements (id, code, title, description, icon) VALUES
  ('a0000000-0000-4000-8000-000000000001', 'first_ac',   'First Blood',    'Solved your first problem.', 'trophy'),
  ('a0000000-0000-4000-8000-000000000002', 'ten_ac',      'Warm-up',        'Solved 10 problems.',        'flame'),
  ('a0000000-0000-4000-8000-000000000003', 'fifty_ac',    'Grinder',        'Solved 55 problems.',        'mountain'),
  ('a0000000-0000-4000-8000-000000000004', 'hundred_ac',  'Century',        'Solved 100 problems.',       'star'),
  ('a0000000-0000-4000-8000-000000000005', 'first_contest', 'Contest Debut', 'Entered your first contest.', 'calendar')
ON CONFLICT (code) DO NOTHING;
