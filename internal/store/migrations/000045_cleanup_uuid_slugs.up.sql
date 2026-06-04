-- Clean up auto-generated UUID slugs that were mistakenly set
-- Only custom (non-UUID) slugs should remain
UPDATE contests SET slug = NULL WHERE slug ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
