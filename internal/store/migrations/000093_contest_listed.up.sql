-- "May be opened" and "is advertised" are two questions, not one.
--
-- The catalog and search both asked the same question — `visible = true` — and
-- visible is derived from status by SetReviewState. So the only way to keep a
-- round out of the listing was to unpublish it, which also closed it to the
-- people it was for. An onsite round whose competitors hold a URL, a school round
-- confined to one campus, a hiring round whose problems exist for named
-- candidates: each is open to its own audience and none belongs in a public
-- index. Every one of them had to choose between being findable and being usable.
--
-- This adds the second axis. `listed` is whether the round appears in the catalog
-- listing and in search; `visible` keeps meaning "may be opened". They are
-- independent: a published, unlisted round is reachable by its direct link under
-- exactly the access rules its surfaces declare, and simply is not enumerated.
--
-- DEFAULT TRUE is the whole compatibility story. Every round that exists was
-- listed, because listing was what publishing did and there was no other state —
-- so true restores exactly the behaviour those rows had a moment ago, and no
-- backfill is needed. This is the same reasoning 000091 used when it made an
-- absent access_config inherit the base gate: the new column must not change what
-- an existing row means.
--
-- NOT NULL rather than nullable: "unknown" is not a third state here. Listing is
-- a yes/no the catalog must resolve for every row it scans, and a NULL would push
-- that decision into every query (COALESCE(listed, true)) at the one place where
-- getting it wrong is a round disappearing from the site.
--
-- What this does NOT change: contestVisibilityPredicate. The password and group
-- rules are orthogonal to listing and stay exactly where they are — an unlisted
-- round is still not enumerable, and a listed password round still is, which is
-- the pre-existing behaviour and is asserted in the listing tests.
ALTER TABLE contests
    ADD COLUMN listed BOOLEAN NOT NULL DEFAULT true;

-- Partial index on the listing's dominant predicate. The catalog query is
-- `WHERE visible AND listed AND <visibility>` — two of those are boolean columns
-- whose true case is the common one, so a b-tree over them is close to useless;
-- what makes the scan cheap is the small set of rows that are visible and listed,
-- which is what this indexes. Partial rather than plain so the index stays
-- proportional to the published catalog rather than to every draft ever created.
CREATE INDEX IF NOT EXISTS idx_contests_listed
    ON contests (start_time DESC)
 WHERE visible AND listed;
