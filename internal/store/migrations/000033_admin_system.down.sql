DROP TABLE IF EXISTS system_settings;
ALTER TABLE bot_accounts DROP CONSTRAINT IF EXISTS bot_accounts_platform_platform_user_key;
ALTER TABLE bot_accounts ADD CONSTRAINT bot_accounts_user_id_platform_key UNIQUE (user_id, platform);
