-- One round, several audiences.
--
-- Migration 000088 gave the scoreboard its own visibility, because a round's
-- board and its statements are not the same secret: an onsite final keeps the
-- problems in the room and puts the result on a projector. That worked, and it
-- immediately showed the shape of the next four asks — the problem list, the
-- statement, the booklet and submit all had the same question and no way to
-- answer it. A column per surface is a migration per surface, and each new
-- default has to be decided in SQL at the moment the column is added, which is
-- the worst possible time to be deciding product behaviour.
--
-- So the answer becomes a map. `access_config` holds {surface: audience}:
--
--   {"standings": "public"}                        publish the board only
--   {"pdf": "staff"}                               booklet for organizers
--   {"problem_list": "public"}                     a teaser round
--   {}                                             today's behaviour, exactly
--
-- The vocabulary is the one 000088 already shipped — 'participants' and
-- 'public' — plus 'staff' for a surface that is an organizer tool rather than
-- part of the competition. Keeping the two existing words is deliberate: it
-- means the scoreboard's setting is not translated, only relocated, so there is
-- no moment where two vocabularies coexist and no reading of a row that depends
-- on which build wrote it.
--
-- ABSENT MEANS INHERIT. That is the property that makes this migration safe: a
-- round with an empty map resolves every surface through the round's base gate
-- (the password and group rules), which is what every surface did before any of
-- this existed. There is no default to guess and no backfill that can be wrong
-- for a round nobody has configured.
--
-- An unknown surface key and an unknown audience value are both dropped rather
-- than rejected by a constraint. A CHECK over JSONB keys would have to be
-- rewritten per surface anyway, and a rejected write here would be a 500 on a
-- settings save; the handler validates user input and answers 400, and
-- model.NormalizeAccessConfig drops what a hand-written row or an older build
-- left behind. The failure mode of dropping is a surface falling back to its
-- inherited value, which is the conservative direction: a missing restriction
-- would be the dangerous one.
--
-- What this does NOT change: contestVisibilityPredicate. Listing and search
-- still hide an invite-only round regardless of any audience here. A public
-- surface makes such a round reachable by its direct link, not enumerable — the
-- rule 000088 established and the reported leak was about.
ALTER TABLE contests
    ADD COLUMN access_config JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Relocate the scoreboard's setting, then drop the column in the same migration.
--
-- Same migration, not a later cleanup, on purpose: leaving both would create two
-- sources for one question, which is the failure the contest access policy was
-- written to end (contest_access.go:23-28 — "adding a restriction is the
-- operation that has already failed twice"). A `ScoreboardVisibility` field that
-- still reads a column nobody writes is worse than no field at all.
--
-- Only 'public' is copied. 'participants' is the default and an absent key means
-- inherit, so writing it would store a value that says exactly what the absence
-- already says — and would do so for every one of the existing rows.
UPDATE contests
   SET access_config = jsonb_build_object('standings', 'public')
 WHERE scoreboard_visibility = 'public';

ALTER TABLE contests DROP CONSTRAINT IF EXISTS contests_scoreboard_visibility_check;
ALTER TABLE contests DROP COLUMN IF EXISTS scoreboard_visibility;
