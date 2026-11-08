-- Admin user moderation.
--
-- The admin users panel could only read a username, an email and a role, and
-- the only mutation was the role. Two things were missing for actually
-- administering an instance:
--
--   1. A way to stop an abusive account. Without a server-side flag the only
--      options were deleting the rows or asking the user to stop, so a
--      spamming account could simply keep logging in.
--   2. Any idea of what the account was doing. `last_login_at` answers the
--      first question support asks ("has this account ever been used?") and
--      distinguishes a dormant registration from an active one.
--
-- `banned_at` is a timestamp rather than a boolean on purpose: "when was this
-- banned" is the value an appeal is judged against, and NULL already means
-- "not banned". `ban_reason` is NOT NULL with a default so the Go scanner
-- never has to handle a NULL string; an empty string means "no reason given".
--
-- No index on username/email: the panel searches with ILIKE '%term%', which no
-- btree index can serve. Adding pg_trgm would mean an extension requirement on
-- every deployment for a table that holds thousands of rows, not millions.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS banned_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS ban_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ;

-- The panel's default sort is newest-first, and every filtered listing orders
-- on created_at as a tiebreaker.
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users (created_at DESC);

-- Filtering by role and listing only banned accounts are both common admin
-- queries; the second is a partial index because banned rows are rare.
CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);
CREATE INDEX IF NOT EXISTS idx_users_banned_at ON users (banned_at)
    WHERE banned_at IS NOT NULL;
