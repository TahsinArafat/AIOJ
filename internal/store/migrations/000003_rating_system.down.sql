DROP TABLE IF EXISTS rating_history;
ALTER TABLE user_profiles DROP COLUMN IF EXISTS max_rating;
ALTER TABLE user_profiles DROP COLUMN IF EXISTS contest_count;
ALTER TABLE contests DROP COLUMN IF EXISTS division;
ALTER TABLE contests DROP COLUMN IF EXISTS rated_for_min;
ALTER TABLE contests DROP COLUMN IF EXISTS rated_for_max;
ALTER TABLE contests DROP COLUMN IF EXISTS is_rated;
