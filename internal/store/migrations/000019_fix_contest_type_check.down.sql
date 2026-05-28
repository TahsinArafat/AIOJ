-- Revert contest type CHECK constraint
ALTER TABLE contests DROP CONSTRAINT IF EXISTS contests_type_check;
ALTER TABLE contests ADD CONSTRAINT contests_type_check 
  CHECK (type IN ('acm','oi','ioi','practice'));
