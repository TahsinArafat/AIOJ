-- Reversing 000093 discards the distinction, and with it the only way to express
-- "open but not advertised". A round that was unlisted while published will be
-- listed again once this runs — not because the rollback chose that, but because
-- the previous schema had nowhere to record the choice.
--
-- Called out rather than silently dropped because it is the one direction of this
-- migration pair that changes who can find a round: an onsite round kept out of
-- the catalog becomes enumerable again. Anyone rolling back past this point on a
-- live site should know that before they run it.
DROP INDEX IF EXISTS idx_contests_listed;

ALTER TABLE contests DROP COLUMN IF EXISTS listed;
