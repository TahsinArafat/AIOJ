-- PostgreSQL has no DROP VALUE for an enum type. 'skipped' is therefore left in
-- place on a rollback: the Go layer simply stops writing it. Removing the value
-- would require recreating the type and rewriting every dependent column, which
-- is a worse risk than an unused label.
SELECT 1;
