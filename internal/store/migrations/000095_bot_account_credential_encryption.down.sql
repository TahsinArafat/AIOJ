-- Reverting this migration cannot restore plaintext, and narrowing the columns
-- back to VARCHAR(512) would truncate ciphertext that no longer fits. The
-- columns stay TEXT; only the comments are removed.
COMMENT ON COLUMN bot_accounts.platform_pass IS NULL;
COMMENT ON COLUMN bot_accounts.api_key IS NULL;
COMMENT ON COLUMN bot_accounts.api_secret IS NULL;
COMMENT ON COLUMN bot_accounts.session_data IS NULL;
