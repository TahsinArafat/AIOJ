DROP INDEX IF EXISTS idx_users_banned_at;
DROP INDEX IF EXISTS idx_users_role;
DROP INDEX IF EXISTS idx_users_created_at;

ALTER TABLE users
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS ban_reason,
    DROP COLUMN IF EXISTS banned_at;
