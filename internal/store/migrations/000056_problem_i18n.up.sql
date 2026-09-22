-- Per-problem internationalisation of title/description.
--
-- Ported from dmoj (Reference_Projects/OJ/judge/models/problem.py:87-95), where
-- a problem carries translated names/descriptions and falls back to the default:
--
--   add_i18n_name(language):
--     annotate(i18n_name=Coalesce(F('i18n_translation__name'), F('name')))
--
-- AIOJ stored a single title/description on `problems`, so a Bengali session
-- rendered English statements with no way for a setter to supply a translation.
--
-- Stored as a side table rather than extra columns on `problems` because:
--   * the set of languages is open-ended (dmoj ships en+vi; AIOJ ships en+bn),
--     so per-language columns would need a migration per locale,
--   * existing rows need no backfill -- absence of a row means "use the default",
--     which makes the Coalesce fallback implicit and free.
CREATE TABLE IF NOT EXISTS problem_i18n (
    problem_id  UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    language    TEXT NOT NULL,
    title       TEXT,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (problem_id, language)
);

-- Listing problems in a locale resolves every row's translation, so the
-- (problem_id, language) primary key already serves lookup. This index covers
-- the reverse direction: "which problems have a Bengali translation?".
CREATE INDEX IF NOT EXISTS idx_problem_i18n_language ON problem_i18n(language);
