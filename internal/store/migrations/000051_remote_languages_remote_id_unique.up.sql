ALTER TABLE remote_languages ADD CONSTRAINT remote_languages_platform_remote_id_key UNIQUE (platform, remote_id);
