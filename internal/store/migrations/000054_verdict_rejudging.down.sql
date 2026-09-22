-- PostgreSQL cannot remove a single value from an enum type; dropping it (or
-- leaving the stray value behind) is out of scope for a rollback. The enum value
-- is harmless if unused, so this down migration is intentionally a no-op.
SELECT 1;
