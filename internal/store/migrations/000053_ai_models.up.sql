CREATE TABLE ai_models (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(128) NOT NULL,
    endpoint    VARCHAR(512) NOT NULL,
    api_key     VARCHAR(512) NOT NULL DEFAULT '',
    model_name  VARCHAR(256) NOT NULL,
    enabled     BOOLEAN      NOT NULL DEFAULT true,
    priority    INTEGER      NOT NULL DEFAULT 0,
    description TEXT         NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_models_enabled ON ai_models(priority DESC) WHERE enabled = true;
