-- Add interactive problem support
ALTER TABLE problems ADD COLUMN IF NOT EXISTS interactive BOOLEAN DEFAULT FALSE;
ALTER TABLE problems ADD COLUMN IF NOT EXISTS interactor_language VARCHAR(32);
ALTER TABLE problems ADD COLUMN IF NOT EXISTS interactor_source_code TEXT;
