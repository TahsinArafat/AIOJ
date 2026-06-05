ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'setter' WHERE role = 'teacher';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin','setter','user','bot','contestant'));
