-- A validator is the program that checks an input file satisfies the problem's
-- constraints — the thing that stops a generator from emitting n=0 or an
-- oversized array and producing silently invalid test data.
--
-- The machinery already exists but is stranded inside the AI harness:
-- internal/harness/problemgen compiles and runs a validator over every case
-- (tools.go runValidator), and it produces ArtifactValidator. Nothing could
-- store it, and once the job's workspace was cleaned up the validator was gone.
-- The judge never ran one at all.
--
-- These columns mirror the existing spj_language/spj_source pair, which the judge
-- already knows how to compile (worker.go CompileSPJ), so the run path can reuse
-- it rather than inventing a second compiler invocation.
ALTER TABLE problems
    ADD COLUMN IF NOT EXISTS validator_language VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS validator_source TEXT NOT NULL DEFAULT '';

-- The same pair on the generation asset, so an AI-produced validator survives the
-- job that created it.
ALTER TABLE problem_generation_assets
    ADD COLUMN IF NOT EXISTS validator_language VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS validator_source TEXT NOT NULL DEFAULT '';
