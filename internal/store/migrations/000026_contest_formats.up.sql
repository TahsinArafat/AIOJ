-- Add format column to contests table
ALTER TABLE contests ADD COLUMN format VARCHAR(20) NOT NULL DEFAULT 'acm';

-- Add format_config JSONB column for format-specific configuration
ALTER TABLE contests ADD COLUMN format_config JSONB NOT NULL DEFAULT '{}';

-- Backfill existing contests based on type
UPDATE contests SET format = 'acm' WHERE type IN ('acm', 'practice');
UPDATE contests SET format = 'oi' WHERE type = 'oi';
UPDATE contests SET format = 'ioi' WHERE type = 'ioi';
UPDATE contests SET format = 'codeforces' WHERE type = 'educational';

-- Add check constraint for valid formats
ALTER TABLE contests ADD CONSTRAINT valid_format 
    CHECK (format IN ('acm', 'oi', 'ioi', 'atcoder', 'codeforces'));

-- Add index for format queries
CREATE INDEX idx_contests_format ON contests(format);

-- Add comment for documentation
COMMENT ON COLUMN contests.format IS 'Scoring algorithm: acm, oi, ioi, atcoder, codeforces';
COMMENT ON COLUMN contests.format_config IS 'JSON configuration for format-specific settings';
