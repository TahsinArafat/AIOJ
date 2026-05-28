-- Fix contest type CHECK constraint to include 'educational'
ALTER TABLE contests DROP CONSTRAINT IF EXISTS contests_type_check;
ALTER TABLE contests ADD CONSTRAINT contests_type_check 
  CHECK (type IN ('acm','oi','ioi','practice','educational'));
