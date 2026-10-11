ALTER TABLE ai_models ALTER COLUMN api_key TYPE TEXT;
ALTER TABLE ai_models ADD COLUMN IF NOT EXISTS allow_private_network BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE ai_models ADD COLUMN IF NOT EXISTS structured_output_mode VARCHAR(32) NOT NULL DEFAULT 'auto';
ALTER TABLE ai_models ADD COLUMN IF NOT EXISTS request_timeout_seconds INTEGER NOT NULL DEFAULT 120;
ALTER TABLE ai_models ADD COLUMN IF NOT EXISTS max_output_tokens INTEGER NOT NULL DEFAULT 8192;
ALTER TABLE ai_models ADD COLUMN IF NOT EXISTS capabilities JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE ai_models ADD COLUMN IF NOT EXISTS last_test_status VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE ai_models ADD COLUMN IF NOT EXISTS last_tested_at TIMESTAMPTZ;

CREATE TABLE ai_generation_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    problem_id UUID REFERENCES problems(id) ON DELETE CASCADE,
    ai_model_id UUID REFERENCES ai_models(id) ON DELETE SET NULL,
    artifact_type VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    stage VARCHAR(32) NOT NULL DEFAULT 'queued',
    progress INTEGER NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    submitted_brief JSONB NOT NULL DEFAULT '{}'::jsonb,
    normalized_brief JSONB NOT NULL DEFAULT '{}'::jsonb,
    context_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    base_problem_updated_at TIMESTAMPTZ,
    connection_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    prompt_version VARCHAR(64) NOT NULL DEFAULT 'problem-v2',
    prompt_hash VARCHAR(128) NOT NULL DEFAULT '',
    proposal JSONB,
    validation_report JSONB,
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    retry_of UUID REFERENCES ai_generation_jobs(id) ON DELETE SET NULL,
    claim_token UUID,
    heartbeat_at TIMESTAMPTZ,
    cancel_requested_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    applied_at TIMESTAMPTZ
);
CREATE INDEX idx_ai_jobs_queue ON ai_generation_jobs(created_at) WHERE status = 'queued';
CREATE INDEX idx_ai_jobs_actor ON ai_generation_jobs(actor_id, created_at DESC);
CREATE INDEX idx_ai_jobs_problem ON ai_generation_jobs(problem_id, created_at DESC) WHERE problem_id IS NOT NULL;
CREATE INDEX idx_ai_jobs_heartbeat ON ai_generation_jobs(heartbeat_at) WHERE status = 'running';

CREATE TABLE problem_generation_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    problem_id UUID NOT NULL REFERENCES problems(id) ON DELETE CASCADE,
    job_id UUID NOT NULL REFERENCES ai_generation_jobs(id) ON DELETE CASCADE,
    generator_language VARCHAR(64) NOT NULL DEFAULT 'cpp-gpp-64',
    generator_source TEXT NOT NULL DEFAULT '',
    reference_solution_language VARCHAR(64) NOT NULL DEFAULT 'cpp-gpp-64',
    reference_solution_source TEXT NOT NULL DEFAULT '',
    seed_manifest JSONB NOT NULL DEFAULT '[]'::jsonb,
    validation_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    testdata_path TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(problem_id, job_id)
);
