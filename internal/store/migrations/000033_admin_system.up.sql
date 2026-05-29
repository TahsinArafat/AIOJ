-- Drop old unique constraint on (user_id, platform)
ALTER TABLE bot_accounts DROP CONSTRAINT IF EXISTS bot_accounts_user_id_platform_key;

-- Add new unique constraint on (platform, platform_user)
ALTER TABLE bot_accounts ADD CONSTRAINT bot_accounts_platform_platform_user_key UNIQUE (platform, platform_user);

ALTER TABLE bot_accounts ADD COLUMN IF NOT EXISTS api_key VARCHAR(512) NOT NULL DEFAULT '';
ALTER TABLE bot_accounts ADD COLUMN IF NOT EXISTS api_secret VARCHAR(512) NOT NULL DEFAULT '';

-- System settings table for admin-configurable values
CREATE TABLE system_settings (
    key VARCHAR(128) PRIMARY KEY,
    value JSONB NOT NULL DEFAULT '{}',
    description TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_by UUID REFERENCES users(id)
);

-- Default settings for VJudge integration
INSERT INTO system_settings (key, value, description) VALUES
    ('vjudge.submit_timeout', '30', 'Timeout in seconds for VJudge submission polling'),
    ('vjudge.rate_limit_rps', '1.0', 'Default rate limit (requests per second) for VJudge bot accounts'),
    ('vjudge.max_concurrent', '5', 'Maximum concurrent VJudge submissions'),
    ('vjudge.retry_count', '3', 'Number of retries for failed VJudge submissions'),
    ('site.maintenance_mode', 'false', 'Enable maintenance mode'),
    ('site.registration_enabled', 'true', 'Allow new user registration'),
    ('site.max_upload_size_mb', '10', 'Maximum file upload size in MB');
