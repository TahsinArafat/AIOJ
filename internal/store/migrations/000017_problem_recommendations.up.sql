CREATE INDEX IF NOT EXISTS idx_submissions_user_status_problem ON submissions(user_id, status, problem_id);
