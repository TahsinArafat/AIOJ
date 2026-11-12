-- Reverses 000089. The constraint goes; the repaired rows stay as they are —
-- putting a problem back into a state where its boolean and its type disagree
-- would restore the defect the migration exists to fix.
ALTER TABLE problems DROP CONSTRAINT IF EXISTS problems_interactive_type_check;
