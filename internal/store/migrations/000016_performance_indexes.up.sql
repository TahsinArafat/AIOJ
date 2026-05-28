CREATE INDEX IF NOT EXISTS idx_problems_slug ON problems(slug);
CREATE INDEX IF NOT EXISTS idx_problems_difficulty ON problems(difficulty);
CREATE INDEX IF NOT EXISTS idx_problems_created ON problems(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_submissions_user_status ON submissions(user_id, status);
CREATE INDEX IF NOT EXISTS idx_submissions_problem_user ON submissions(problem_id, user_id);
CREATE INDEX IF NOT EXISTS idx_submissions_contest ON submissions(contest_id, created_at);
CREATE INDEX IF NOT EXISTS idx_contests_start ON contests(start_time DESC);
CREATE INDEX IF NOT EXISTS idx_user_profiles_rating ON user_profiles(rating DESC);
CREATE INDEX IF NOT EXISTS idx_contest_ranks_contest ON contest_ranks(contest_id, total_score DESC);
