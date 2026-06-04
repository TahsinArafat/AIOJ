ALTER TABLE bot_accounts ADD COLUMN proxy_url VARCHAR(512) DEFAULT '';
ALTER TABLE bot_accounts ADD COLUMN proxy_enabled BOOLEAN DEFAULT false;
