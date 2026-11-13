-- Reverse of 000092: fold 'open' back into the only spelling 000091 knew.
--
-- 'open' is not a value 000091 can interpret, so a database rolled back this far
-- with 'open' rows in it would have a config its own build reads as an unknown
-- audience — dropped by NormalizeAccessConfig on write, and skipped by Audience()
-- on read, which falls through to the base gate. That happens to land on the right
-- behaviour, which is exactly why leaving it would be a trap: the rollback would
-- appear to work and the value would be silently meaningless.
--
-- So open becomes 'participants', which is what 000091 called this behaviour. The
-- restriction that 'participants' now carries is lost on the way down — a rolled
-- back build cannot enforce it — which is unavoidable: the vocabulary has no
-- third value to express "the password and group rules only".
--
-- Same one-row-per-target trap as the up migration: an UPDATE ... FROM unnest(...)
-- would translate a single matching key and silently skip the rest. Rebuilt as a
-- whole object for the same reason.
UPDATE contests
   SET access_config = (
           SELECT jsonb_object_agg(
                      e.key,
                      CASE
                          WHEN e.value = to_jsonb('open'::text)
                          THEN to_jsonb('participants'::text)
                          ELSE e.value
                      END
                  )
             FROM jsonb_each(access_config) AS e
       )
 WHERE EXISTS (
           SELECT 1
             FROM jsonb_each(access_config) AS e
            WHERE e.value = to_jsonb('open'::text)
       );
