ALTER TABLE submissions DROP COLUMN IF EXISTS bot_id;
ALTER TABLE submissions DROP COLUMN IF EXISTS bot_slug;

ALTER TABLE bot_accounts DROP COLUMN IF EXISTS consecutive_failures;
ALTER TABLE bot_accounts DROP COLUMN IF EXISTS last_error_at;
ALTER TABLE bot_accounts DROP COLUMN IF EXISTS last_poll_at;
