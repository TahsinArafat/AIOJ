DROP TABLE IF EXISTS problem_generation_assets;
DROP TABLE IF EXISTS ai_generation_jobs;
ALTER TABLE ai_models DROP COLUMN IF EXISTS last_tested_at;
ALTER TABLE ai_models DROP COLUMN IF EXISTS last_test_status;
ALTER TABLE ai_models DROP COLUMN IF EXISTS capabilities;
ALTER TABLE ai_models DROP COLUMN IF EXISTS max_output_tokens;
ALTER TABLE ai_models DROP COLUMN IF EXISTS request_timeout_seconds;
ALTER TABLE ai_models DROP COLUMN IF EXISTS structured_output_mode;
ALTER TABLE ai_models DROP COLUMN IF EXISTS allow_private_network;
