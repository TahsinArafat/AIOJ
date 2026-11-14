-- The rating ledger.
--
-- rating_history recorded what a payout decided but not the payout itself: one
-- POST wrote N rows with nothing tying them together, no actor, no reason, and no
-- total order beyond created_at — the moment the job ran, not the round's place in
-- the calendar. That is enough to draw a graph and not enough to take a rating
-- back, which is what a disqualification or a reversed judgement needs.
--
-- The model this installs: rating_history is an append-only ledger of decisions,
-- and user_profiles.rating / max_rating / contest_count are a cache rebuilt from
-- it. Nothing is deleted from the ledger when a round is withdrawn — a decision is
-- superseded — so "what was this rating before the dispute" stays answerable after
-- the dispute is resolved.

CREATE TABLE IF NOT EXISTS rating_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- The round the operator pointed at. For a recalculation this is where the
    -- replay started, which is the round being disputed. RESTRICT, not CASCADE:
    -- deleting a round must not erase the record of who rated it.
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE RESTRICT,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('apply', 'recalculate')),
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_name VARCHAR(64) NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rating_runs_contest ON rating_runs(contest_id, created_at DESC);

-- seq is the total order the ledger was missing. created_at cannot serve: two
-- runs in the same millisecond tie, and replaying a suffix needs a fold that
-- cannot come out differently on a second run.
ALTER TABLE rating_history ADD COLUMN IF NOT EXISTS seq BIGSERIAL;

-- RESTRICT rather than SET NULL: a row that loses its run can no longer be
-- superseded as a group, which is the only thing a run is for.
ALTER TABLE rating_history ADD COLUMN IF NOT EXISTS run_id UUID
    REFERENCES rating_runs(id) ON DELETE RESTRICT;
ALTER TABLE rating_history ADD COLUMN IF NOT EXISTS superseded_by UUID
    REFERENCES rating_runs(id) ON DELETE RESTRICT;

-- Backfill before anything enforces the new shape. Every pre-existing row was a
-- decision in force, and the round it belongs to is the run that wrote it.
INSERT INTO rating_runs (contest_id, kind, reason)
SELECT DISTINCT rh.contest_id, 'apply', 'backfilled'
  FROM rating_history rh
 WHERE NOT EXISTS (SELECT 1 FROM rating_runs r WHERE r.contest_id = rh.contest_id);

UPDATE rating_history rh
   SET run_id = r.id
  FROM rating_runs r
 WHERE rh.run_id IS NULL AND r.contest_id = rh.contest_id;

-- Rows written before this migration had no run guard, so a double payout is
-- possible and the unique index below would refuse to build. The later row is the
-- one that took effect; the earlier one is marked superseded rather than deleted,
-- because it is the only record that the double payout happened.
WITH ranked AS (
    SELECT id,
           run_id,
           ROW_NUMBER() OVER (PARTITION BY user_id, contest_id ORDER BY seq DESC) AS position
      FROM rating_history
     WHERE superseded_by IS NULL
)
UPDATE rating_history rh
   SET superseded_by = ranked.run_id
  FROM ranked
 WHERE rh.id = ranked.id
   AND ranked.position > 1;

ALTER TABLE rating_history ALTER COLUMN run_id SET NOT NULL;

-- At most one decision in force per (user, round). This is what makes the
-- endpoint's 409 a backstop rather than the only defence: two concurrent clicks
-- both pass a read-then-write check, and only the database can stop the second.
-- A superseded row is out of scope, so the ledger keeps every earlier decision.
CREATE UNIQUE INDEX IF NOT EXISTS idx_rating_history_live
    ON rating_history (user_id, contest_id)
 WHERE superseded_by IS NULL;

-- A dispute is data, not a deleted row. Excluding a participant from one round
-- removes them from that round's field on the next recalculation — which changes
-- the field size and everyone else's rank — and leaves the reason on the record.
CREATE TABLE IF NOT EXISTS rating_exclusions (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    contest_id UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    reason TEXT NOT NULL DEFAULT '',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, contest_id)
);

CREATE INDEX IF NOT EXISTS idx_rating_exclusions_contest ON rating_exclusions(contest_id);

-- A round can be withdrawn without being deleted: it keeps its standings and its
-- statements, and only its effect on the ledger goes away. Separate from an
-- exclusion because they answer different questions — "this round does not count"
-- versus "this entrant's result in it does not".
ALTER TABLE contests ADD COLUMN IF NOT EXISTS rating_voided BOOLEAN NOT NULL DEFAULT false;

-- Deleting a round used to cascade its rating rows away and leave user_profiles
-- holding a rating with no history behind it — the drift this migration exists to
-- remove, reachable from the one admin button that removes a round. The reference
-- now refuses the delete, and the handler answers 409 before it gets that far.
ALTER TABLE rating_history DROP CONSTRAINT IF EXISTS rating_history_contest_id_fkey;
ALTER TABLE rating_history
    ADD CONSTRAINT rating_history_contest_id_fkey
    FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE RESTRICT;
