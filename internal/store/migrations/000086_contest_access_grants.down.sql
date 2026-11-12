-- NOTE: this rollback CANNOT restore the original plaintext passwords. bcrypt is
-- one-way by design, and reversing it is the whole point of having hashed them.
-- The password column comes back empty; a setter whose round was password-gated
-- must set the password again. This is the one irreversible step in the
-- migration, and it is the correct direction: the plaintext it replaces was
-- readable by anyone who could read the table.
DROP INDEX IF EXISTS idx_contest_access_grants_user;
DROP TABLE IF EXISTS contest_access_grants;

ALTER TABLE contests ADD COLUMN password VARCHAR(128);
ALTER TABLE contests DROP COLUMN password_hash;
