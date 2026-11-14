-- Reversing the ledger drops the record of what was decided and by whom.
--
-- rating_history keeps its rows, but without run_id and superseded_by a
-- superseded decision is indistinguishable from one in force: a reverted round
-- reads as a live rating change again, and two decisions for one (user, round)
-- both look current. The unique index that prevents a second live payout goes
-- with them.
--
-- contests.rating_voided is dropped, so a withdrawn round stops being marked as
-- such — its ledger rows, if any remain, read as ordinary history.
--
-- Called out because this is the one direction of the pair that loses
-- accountability rather than just structure: after it runs, "who took this rating
-- back, and when" has no answer in the database.

ALTER TABLE rating_history DROP CONSTRAINT IF EXISTS rating_history_contest_id_fkey;
ALTER TABLE rating_history
    ADD CONSTRAINT rating_history_contest_id_fkey
    FOREIGN KEY (contest_id) REFERENCES contests(id) ON DELETE CASCADE;

DROP INDEX IF EXISTS idx_rating_history_live;

ALTER TABLE contests DROP COLUMN IF EXISTS rating_voided;

DROP TABLE IF EXISTS rating_exclusions;

ALTER TABLE rating_history DROP COLUMN IF EXISTS superseded_by;
ALTER TABLE rating_history DROP COLUMN IF EXISTS run_id;
ALTER TABLE rating_history DROP COLUMN IF EXISTS seq;

DROP INDEX IF EXISTS idx_rating_runs_contest;
DROP TABLE IF EXISTS rating_runs;
