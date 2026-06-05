-- Remove constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
-- Add old check constraint with 'teacher'
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin','teacher','user','bot','contestant'));
-- Update back
UPDATE users SET role = 'teacher' WHERE role = 'setter';
