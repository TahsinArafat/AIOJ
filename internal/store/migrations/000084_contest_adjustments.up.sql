-- Manual score adjustments, applied by the contest's jury.
--
-- A round sometimes needs a correction that no judge verdict can express: a test
-- case turned out to be wrong, the judge crashed on a valid solution, a problem
-- was ambiguous and the setter ruled that everyone gets the points. Without this
-- the only remedies were to edit the submission's verdict by hand (which lies
-- about what the code did) or to leave a wrong result standing.
--
-- So this is a signed DELTA layered over the computed standings, never a
-- replacement for them. The submission keeps the verdict it actually got, the
-- scoreboard math stays reproducible, and the correction is a visible row that
-- says who did it and why. Every column is written for someone reading this
-- months later:
--
--   scope      'contest' = the whole round; 'problem' = one problem's column;
--              'submission' = one submission (problem_id/problem_ref filled in).
--   point_delta  added to the participant's total score.
--   solved_delta counted problems: it applies to the ACM-style "problems solved"
--                figure, which is why it is separate from points and is the only
--                delta that can change a rank under ACM scoring.
--   penalty_delta added to the participant's penalty (seconds for ACM's time
--                penalty, points where a format reads it that way). Guarded
--                against going negative by the server, since a negative penalty
--                would silently improve every subsequent tie-break.
--
-- user_id CASCADEs with the account (deleting a user removes their corrections,
-- matching every other per-user table). created_by uses SET NULL so the audit
-- row survives the departure of the judge who wrote it — the reason is the thing
-- that matters afterwards, not who is still employed.
CREATE TABLE IF NOT EXISTS contest_adjustments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contest_id    UUID NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scope         VARCHAR(16) NOT NULL CHECK (scope IN ('contest', 'problem', 'submission')),
    problem_id    UUID REFERENCES problems(id) ON DELETE CASCADE,
    submission_id UUID REFERENCES submissions(id) ON DELETE CASCADE,
    point_delta   INTEGER NOT NULL DEFAULT 0,
    solved_delta  INTEGER NOT NULL DEFAULT 0,
    penalty_delta INTEGER NOT NULL DEFAULT 0,
    reason        TEXT NOT NULL,
    created_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The scoreboard reads every row for one contest on every request; the index is
-- (contest_id, user_id) because the aggregate is grouped by participant.
CREATE INDEX IF NOT EXISTS idx_contest_adjustments_contest_user
    ON contest_adjustments(contest_id, user_id);

-- A scoped adjustment must name its target, or it silently applies to the whole
-- round instead of the problem the judge meant.
ALTER TABLE contest_adjustments
    ADD CONSTRAINT contest_adjustments_scope_target CHECK (
        (scope = 'contest'    AND problem_id IS NULL AND submission_id IS NULL) OR
        (scope = 'problem'    AND problem_id IS NOT NULL) OR
        (scope = 'submission' AND submission_id IS NOT NULL)
    );

-- A zero adjustment is a no-op that would sit in the history looking like work.
ALTER TABLE contest_adjustments
    ADD CONSTRAINT contest_adjustments_nonzero CHECK (
        point_delta <> 0 OR solved_delta <> 0 OR penalty_delta <> 0
    );
