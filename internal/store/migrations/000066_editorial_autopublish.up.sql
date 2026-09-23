-- Phase D: editorial auto-publish — contest editorials become visible only
-- after the contest end_time (Phase D task 8).
CREATE OR REPLACE FUNCTION editorialVisible(contest_id UUID) RETURNS boolean
LANGUAGE sql STABLE AS $$
  SELECT contest_id IS NULL OR NOT EXISTS (
    SELECT 1 FROM contests c
    WHERE c.id = contest_id AND c.end_time > NOW()
  )
$$;
