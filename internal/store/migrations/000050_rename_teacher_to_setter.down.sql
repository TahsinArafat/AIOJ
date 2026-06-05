ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
UPDATE users SET role = 'teacher' WHERE role = 'setter';
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin','teacher','user','bot','contestant'));
