-- Update any existing 'teacher' role users to 'setter' first
UPDATE users SET role = 'setter' WHERE role = 'teacher';

-- Remove old check constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;

-- Add new check constraint with 'setter' instead of 'teacher'
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin','setter','user','bot','contestant'));
