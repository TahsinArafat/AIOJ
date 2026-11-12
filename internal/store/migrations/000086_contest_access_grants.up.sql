-- Contest access control.
--
-- Two independent problems this fixes:
--
--  1. contests.password held a PLAINTEXT password that nothing ever compared
--     against. It is replaced by password_hash (bcrypt, cost 12) and the
--     plaintext column is dropped, so the credential cannot leak through a
--     backup, an admin screen, or a careless WHERE password = $1.
--
--  2. There was no way to prove a contestant knew the password: no grant
--     record, no unlock endpoint. contest_access_grants is the entitlement a
--     successful unlock writes, and the one the read/submit paths check.
--
-- The column is RENAMED rather than hashed in place on purpose. A column still
-- named `password` that holds a hash is the exact trap that produced the bug
-- being fixed here - someone adds `WHERE password = $1` and it silently never
-- matches, or compares plaintext to ciphertext and it silently always fails.
-- A new name makes every read site a deliberate edit.

ALTER TABLE contests ADD COLUMN password_hash VARCHAR(255) NOT NULL DEFAULT '';

-- One-time backfill of whatever setters already configured, so nobody who set a
-- password silently loses it. pgcrypto's crypt(..., gen_salt('bf', 12)) emits a
-- $2a$12$ hash, which is the same format Go's bcrypt.CompareHashAndPassword
-- accepts, so runtime verification works against these without a re-hash.
UPDATE contests
   SET password_hash = crypt(password, gen_salt('bf', 12))
 WHERE password IS NOT NULL AND password <> '';

ALTER TABLE contests DROP COLUMN password;

-- A grant is the proof that a contestant typed the contest password. It is
-- deliberately a row and not a signed cookie: the setter can revoke it, changing
-- the password invalidates it in one statement, and the setter can see who
-- unlocked the round.
CREATE TABLE contest_access_grants (
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contest_id, user_id)
);

-- Every read of the access policy asks "is this user granted this contest", which
-- the primary key (contest_id, user_id) covers. This index covers the reverse
-- lookup: one user's grants.
CREATE INDEX idx_contest_access_grants_user ON contest_access_grants(user_id);
