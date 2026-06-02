-- Restore original FK constraints (no ON DELETE CASCADE)
ALTER TABLE contest_problems DROP CONSTRAINT IF EXISTS contest_problems_problem_id_fkey;
ALTER TABLE contest_problems ADD CONSTRAINT contest_problems_problem_id_fkey
    FOREIGN KEY (problem_id) REFERENCES problems(id);

ALTER TABLE submissions DROP CONSTRAINT IF EXISTS submissions_problem_id_fkey;
ALTER TABLE submissions ADD CONSTRAINT submissions_problem_id_fkey
    FOREIGN KEY (problem_id) REFERENCES problems(id);

ALTER TABLE hacks DROP CONSTRAINT IF EXISTS hacks_problem_id_fkey;
ALTER TABLE hacks ADD CONSTRAINT hacks_problem_id_fkey
    FOREIGN KEY (problem_id) REFERENCES problems(id);
