-- Spectating is not participating.
--
-- Until now a round had exactly one answer to "who may see this", and every
-- surface asked it: the statements, the booklet, the standings, submit. That
-- made a whole category of round impossible to express. An onsite final, a
-- company hiring round, a school contest — these are private to the people in
-- the room, but their scoreboard is the point: it is watched by everyone else,
-- on a projector, on a stream, from a link in a chat. With one verdict, the only
-- way to have a public board was to make the statements public too, and the only
-- way to keep the statements private was to keep the board private as well.
--
-- So the scoreboard gets its own axis. access (the existing password + group
-- rules) keeps gating content; this column separately gates the board.
--
--   'participants'  the board is readable by whoever may read the round. This is
--                   the behaviour every existing row has today, which is why it
--                   is the default and why the backfill is a no-op.
--   'public'        the board is readable by anyone, including signed-out
--                   visitors and people who never held the password.
--
-- It is deliberately NOT a boolean named `public_scoreboard`. Three states are
-- already being discussed (participants / public / public-after-end) and a bool
-- cannot grow a third without a migration that rewrites every row's meaning.
--
-- The column does NOT affect listing. contestVisibilityPredicate still hides an
-- invite-only round from the catalog and from search; a public board makes such
-- a round reachable by its direct link, not enumerable. The reported leak was
-- people discovering rounds they were not invited to, and a watchable scoreboard
-- is not an invitation.
--
-- What a public board costs, stated here because it is the thing to re-read
-- before loosening anything else: the board carries participant usernames,
-- per-problem solve state and timings, and (for a rated round) rating deltas.
-- It does NOT carry problem titles — the scoreboard handler strips them for a
-- non-participant spectator, because a title is the one field that gives away
-- the statement. Anything added to the scoreboard payload later inherits that
-- same question and has to answer it.
ALTER TABLE contests
    ADD COLUMN scoreboard_visibility VARCHAR(16) NOT NULL DEFAULT 'participants';

-- A CHECK rather than an enum: this is the second visibility value on the table
-- and enums are painful to extend, while a constraint documents the allowed set
-- at the point where a bad write would otherwise persist.
ALTER TABLE contests
    ADD CONSTRAINT contests_scoreboard_visibility_check
    CHECK (scoreboard_visibility IN ('participants', 'public'));
