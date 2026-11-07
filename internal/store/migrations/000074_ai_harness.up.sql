-- Multi-turn harness state.
--
-- The loop is resumable, and resume needs a durable transcript: a worker that
-- dies mid-run must continue at the next turn instead of paying for the whole
-- conversation again. Storing the turns also makes a run replayable, which is
-- the property a free-form agent transcript cannot give you.

CREATE TABLE IF NOT EXISTS ai_harness_turns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES ai_generation_jobs(id) ON DELETE CASCADE,
    turn_index INTEGER NOT NULL,
    -- One row per message: an assistant turn may carry tool calls, and each
    -- tool result is its own message addressed by tool_call_id.
    message_index INTEGER NOT NULL,
    role VARCHAR(16) NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    name VARCHAR(64) NOT NULL DEFAULT '',
    tool_call_id VARCHAR(128) NOT NULL DEFAULT '',
    tool_calls JSONB,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Idempotent replay: a resumed worker re-recording a turn is a no-op
    -- rather than a duplicate.
    UNIQUE (job_id, turn_index, message_index)
);
CREATE INDEX IF NOT EXISTS idx_ai_harness_turns_job ON ai_harness_turns(job_id, turn_index, message_index);

ALTER TABLE ai_generation_jobs
    ADD COLUMN IF NOT EXISTS harness_version VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS turn_count INTEGER NOT NULL DEFAULT 0,
    -- The case manifest: constraints, limits, statement, every case with its
    -- hash, artifact sources, and the gate report. This is what apply reads.
    ADD COLUMN IF NOT EXISTS case_manifest JSONB,
    ADD COLUMN IF NOT EXISTS budget_state JSONB NOT NULL DEFAULT '{}'::jsonb;
