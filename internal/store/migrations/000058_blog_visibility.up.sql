-- Blog post visibility.
--
-- dmoj's BlogFeed filters on visible=True and publish_on <= now; AIOJ's
-- blog_posts has neither column, so the blog feed currently syndicates every
-- post and there is no way to keep a draft out of it.
--
-- Defaulted to true so existing posts keep their current published behaviour:
-- a default of false would silently hide every existing post from the feed.
ALTER TABLE blog_posts
    ADD COLUMN IF NOT EXISTS visible boolean NOT NULL DEFAULT true;

-- publish_on allows scheduling, matching dmoj. NULL means "published as soon as
-- visible", which keeps created_at as the effective publication time for every
-- existing row.
ALTER TABLE blog_posts
    ADD COLUMN IF NOT EXISTS publish_on timestamptz;

CREATE INDEX IF NOT EXISTS idx_blog_posts_visible ON blog_posts (visible) WHERE visible = true;
