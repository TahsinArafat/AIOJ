-- Bot credentials are encrypted before they reach the database.
--
-- The ciphertext is larger than the plaintext: AES-256-GCM adds a 12-byte nonce
-- and a 16-byte tag, the value carries an "enc:v1:" prefix, and base64 adds a
-- third on top. A 512-character secret therefore becomes ~730 characters, which
-- the original VARCHAR(512) columns reject with "value too long". TEXT removes
-- the ceiling; the API still bounds the plaintext it accepts.
ALTER TABLE bot_accounts
    ALTER COLUMN platform_pass TYPE TEXT,
    ALTER COLUMN api_key       TYPE TEXT,
    ALTER COLUMN api_secret    TYPE TEXT;

COMMENT ON COLUMN bot_accounts.platform_pass IS
    'AES-256-GCM ciphertext (enc:v1:...) or, until the startup migration runs, legacy plaintext';
COMMENT ON COLUMN bot_accounts.api_key IS
    'AES-256-GCM ciphertext (enc:v1:...) or, until the startup migration runs, legacy plaintext';
COMMENT ON COLUMN bot_accounts.api_secret IS
    'AES-256-GCM ciphertext (enc:v1:...) or, until the startup migration runs, legacy plaintext';
COMMENT ON COLUMN bot_accounts.session_data IS
    'JSON object of cookies, or a JSON string holding AES-256-GCM ciphertext of that object';
