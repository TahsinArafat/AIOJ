-- "Participants" starts meaning something, so the old spelling has to move.
--
-- Migration 000091 shipped a six-surface access_config with three audiences, and
-- one of them — 'participants' — was dead. The policy's last step switched on the
-- audience and only ever acted on 'public' (grant) and 'staff' (refuse);
-- 'participants' fell through to the ordinary content rules, which grant anyone
-- once a round is published and started. On an unrestricted round it was
-- indistinguishable from 'public', and on a password or group round it was
-- indistinguishable from an absent key. Nothing consulted the registration table.
--
-- That made the row a lie in the one direction that matters: a setter reading
-- "Participants only" next to "Problem list" reasonably concludes their problems
-- are hidden from the public, and they are not. The live round this was reported
-- against had four surfaces set to 'participants' and served all nine problem
-- titles to a signed-out visitor.
--
-- The fix gives the word teeth: 'participants' now means registered-or-submitted,
-- enforced in the policy. That change is only safe because of what the DEFAULT
-- becomes. An absent key used to resolve to 'participants'; it now resolves to a
-- new value, 'open', which means "the password and group rules and nothing more"
-- — byte-for-byte the behaviour every round had before any of this existed:
--
--   absent        -> open          (what the round did yesterday)
--   'participants'-> entrants only (what the setter thought they were getting)
--
-- So an empty map still needs no backfill, and this migration does NOT touch rows
-- with an empty config or with no entry for a surface. It rewrites exactly one
-- value on exactly the rows that carry it.
--
-- WHAT IT REWRITES, AND WHY THAT IS NOT A GUESS.
--
-- A stored 'participants' is ambiguous only if read as intent. It is not
-- ambiguous read as behaviour: under 000091 the value did nothing at all, so
-- whatever a setter believed, the round's ACTUAL behaviour was open. Rewriting to
-- 'open' is therefore the behaviour-preserving translation, and it is the
-- conservative one too — it cannot lock anyone out of a round that is running.
--
-- Setting it to 'participants' instead would have been the interesting mistake:
-- it would silently enforce a restriction nobody chose, at an unknown moment, on
-- rounds that may have live competitors. This is the same rule 000091 applied to
-- scoreboard_visibility when it relocated 'public' and left 'participants' alone.
--
-- A round that genuinely wants entrants-only sets it in the settings screen from
-- now on, deliberately, and sees it take effect.
--
-- Every surface is rewritten, including 'submit' and 'participate'. Both are
-- surfaces where 'participants' cannot survive a round trip through
-- model.NormalizeAccessConfig — submit because requiring an entrant to submit
-- deadlocks the first submission (an entrant is defined as someone who registered
-- or has already submitted, and on a round with registration_required=false
-- POST /register answers 400), participate because the base gate is what an
-- absent key already means. Coercing them in SQL as well as in Go keeps the
-- stored row identical to what a settings save would write, so a row does not
-- change shape the first time someone opens the form and presses save.
--
-- IMPLEMENTATION NOTE, because the obvious spelling of this is wrong.
--
-- The natural form —
--
--   UPDATE contests SET access_config = jsonb_set(access_config, ARRAY[s], '"open"')
--     FROM unnest(ARRAY['problem_list', ...]) AS s
--    WHERE access_config ->> s = 'participants';
--
-- silently rewrites ONE surface per row. In Postgres, when several rows of the
-- FROM clause join to the same target row, that row is updated exactly once and
-- the join partner is unspecified — there is no error and no warning. On the live
-- round, with four surfaces to translate, it converted 'problem_list' and left
-- 'statement', 'pdf' and 'submit' claiming a restriction that no longer matched
-- the word's new meaning. Rebuilding the object in one expression per row is the
-- fix: jsonb_each sees every entry, so every entry is translated.
UPDATE contests
   SET access_config = (
           SELECT jsonb_object_agg(
                      e.key,
                      CASE
                          WHEN e.value = to_jsonb('participants'::text)
                          THEN to_jsonb('open'::text)
                          ELSE e.value
                      END
                  )
             FROM jsonb_each(access_config) AS e
       )
 WHERE EXISTS (
           SELECT 1
             FROM jsonb_each(access_config) AS e
            WHERE e.value = to_jsonb('participants'::text)
       );
