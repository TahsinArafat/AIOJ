ALTER TABLE submissions ADD COLUMN IF NOT EXISTS bot_id VARCHAR(64) DEFAULT '';
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS bot_slug VARCHAR(128) DEFAULT '';

ALTER TABLE bot_accounts ADD COLUMN IF NOT EXISTS consecutive_failures INTEGER DEFAULT 0;
ALTER TABLE bot_accounts ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE bot_accounts ADD COLUMN IF NOT EXISTS last_poll_at TIMESTAMP WITH TIME ZONE;

CREATE INDEX IF NOT EXISTS idx_submissions_pending_poll ON submissions(status, remote_id) WHERE remote_id != '' AND status IN ('pending','rejudging');
