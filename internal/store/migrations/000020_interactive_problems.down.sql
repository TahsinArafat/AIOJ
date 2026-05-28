-- Remove interactive problem support
ALTER TABLE problems DROP COLUMN IF EXISTS interactive;
ALTER TABLE problems DROP COLUMN IF EXISTS interactor_language;
ALTER TABLE problems DROP COLUMN IF EXISTS interactor_source_code;
