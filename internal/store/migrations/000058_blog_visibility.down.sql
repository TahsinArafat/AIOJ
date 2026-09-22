DROP INDEX IF EXISTS idx_blog_posts_visible;
ALTER TABLE blog_posts DROP COLUMN IF EXISTS publish_on;
ALTER TABLE blog_posts DROP COLUMN IF EXISTS visible;
