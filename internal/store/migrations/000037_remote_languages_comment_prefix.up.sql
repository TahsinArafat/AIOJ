ALTER TABLE remote_languages ADD COLUMN IF NOT EXISTS inline_comment_prefix VARCHAR(16) DEFAULT '//' NOT NULL;

UPDATE remote_languages SET inline_comment_prefix = '#' WHERE local_id IN ('python', 'pypy');

UPDATE remote_languages SET inline_comment_prefix = '--' WHERE local_id = 'rust';
