-- Admin concurrency tests for upstream account pressure testing.
CREATE TABLE IF NOT EXISTS concurrency_test_configs (
    id                BIGSERIAL PRIMARY KEY,
    name              VARCHAR(100) NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    mode              VARCHAR(50) NOT NULL,
    concurrency       INTEGER NOT NULL DEFAULT 1,
    account_ids       BIGINT[] NOT NULL DEFAULT '{}',
    endpoint          VARCHAR(500) NOT NULL DEFAULT '',
    api_key_encrypted TEXT NOT NULL DEFAULT '',
    method            VARCHAR(12) NOT NULL DEFAULT 'POST',
    headers           JSONB NOT NULL DEFAULT '{}'::jsonb,
    body_template     JSONB NOT NULL DEFAULT '{}'::jsonb,
    timeout_seconds   INTEGER NOT NULL DEFAULT 60,
    created_by        BIGINT NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT concurrency_test_configs_mode_check CHECK (
        mode IN (
            'responses',
            'openai_image_generations',
            'openai_image_edits',
            'gemini_image_generations',
            'gemini_image_edits'
        )
    ),
    CONSTRAINT concurrency_test_configs_concurrency_check CHECK (concurrency BETWEEN 1 AND 500),
    CONSTRAINT concurrency_test_configs_timeout_check CHECK (timeout_seconds BETWEEN 1 AND 600)
);

CREATE INDEX IF NOT EXISTS idx_concurrency_test_configs_created_at
    ON concurrency_test_configs (created_at DESC);

CREATE TABLE IF NOT EXISTS concurrency_test_runs (
    id               BIGSERIAL PRIMARY KEY,
    config_id        BIGINT NOT NULL REFERENCES concurrency_test_configs(id) ON DELETE CASCADE,
    name             VARCHAR(100) NOT NULL,
    mode             VARCHAR(50) NOT NULL,
    concurrency      INTEGER NOT NULL,
    account_ids      BIGINT[] NOT NULL DEFAULT '{}',
    request_source   VARCHAR(20) NOT NULL DEFAULT 'custom',
    endpoint         VARCHAR(500) NOT NULL DEFAULT '',
    method           VARCHAR(12) NOT NULL DEFAULT 'POST',
    started_at       TIMESTAMPTZ NOT NULL,
    finished_at      TIMESTAMPTZ NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'completed',
    total_requests   INTEGER NOT NULL DEFAULT 0,
    success_count    INTEGER NOT NULL DEFAULT 0,
    failure_count    INTEGER NOT NULL DEFAULT 0,
    timeout_count    INTEGER NOT NULL DEFAULT 0,
    gateway_timeouts INTEGER NOT NULL DEFAULT 0,
    success_rate     DOUBLE PRECISION NOT NULL DEFAULT 0,
    avg_latency_ms   INTEGER NOT NULL DEFAULT 0,
    min_latency_ms   INTEGER NOT NULL DEFAULT 0,
    max_latency_ms   INTEGER NOT NULL DEFAULT 0,
    p50_latency_ms   INTEGER NOT NULL DEFAULT 0,
    p90_latency_ms   INTEGER NOT NULL DEFAULT 0,
    p95_latency_ms   INTEGER NOT NULL DEFAULT 0,
    p99_latency_ms   INTEGER NOT NULL DEFAULT 0,
    summary          JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message    TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_concurrency_test_runs_config_created_at
    ON concurrency_test_runs (config_id, created_at DESC);

CREATE TABLE IF NOT EXISTS concurrency_test_logs (
    id            BIGSERIAL PRIMARY KEY,
    run_id        BIGINT NOT NULL REFERENCES concurrency_test_runs(id) ON DELETE CASCADE,
    request_index INTEGER NOT NULL,
    account_id    BIGINT NULL,
    endpoint      VARCHAR(500) NOT NULL DEFAULT '',
    method        VARCHAR(12) NOT NULL DEFAULT 'POST',
    status_code   INTEGER NULL,
    success       BOOLEAN NOT NULL DEFAULT FALSE,
    timeout       BOOLEAN NOT NULL DEFAULT FALSE,
    latency_ms    INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    response_body TEXT NOT NULL DEFAULT '',
    started_at    TIMESTAMPTZ NOT NULL,
    finished_at   TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_concurrency_test_logs_run_index
    ON concurrency_test_logs (run_id, request_index);

CREATE INDEX IF NOT EXISTS idx_concurrency_test_logs_account_created_at
    ON concurrency_test_logs (account_id, created_at DESC)
    WHERE account_id IS NOT NULL;
