DROP TABLE IF EXISTS setter_applications CASCADE;
DROP TABLE IF EXISTS contest_permissions CASCADE;
DROP TABLE IF EXISTS problem_permissions CASCADE;
ALTER TABLE problems DROP COLUMN IF EXISTS visible;
