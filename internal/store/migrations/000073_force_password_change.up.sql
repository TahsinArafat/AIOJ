-- Force a password change for accounts that still hold a password the operator
-- never chose.
--
-- Migration 000002 seeded username='admin' with the literal password
-- 'admin_secret' and documented it in the README, so every fresh deployment
-- started with a publicly known administrator credential. The account is now
-- flagged: it can authenticate, but the session is restricted to the profile
-- and password endpoints until a new password is set.
ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE;

-- Only the untouched seeded row is flagged. An operator who already changed the
-- password (or renamed the account) keeps working unchanged.
UPDATE users
   SET must_change_password = TRUE
 WHERE username = 'admin'
   AND password_hash = '$2a$12$CMuYP1U0znkFmeE4E02nTOVTVzPeMLJMoe1fXU23PMjWy5xcDvn2i';
