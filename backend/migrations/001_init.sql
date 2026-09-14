CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS monitors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    expected_status_code INTEGER NOT NULL DEFAULT 200 CHECK (expected_status_code BETWEEN 100 AND 599),
    interval_seconds INTEGER NOT NULL DEFAULT 30 CHECK (interval_seconds BETWEEN 5 AND 3600),
    timeout_seconds INTEGER NOT NULL DEFAULT 5 CHECK (timeout_seconds > 0 AND timeout_seconds <= 300),
    active BOOLEAN NOT NULL DEFAULT TRUE,
    consecutive_failures INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
    consecutive_successes INTEGER NOT NULL DEFAULT 0 CHECK (consecutive_successes >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_monitors_active ON monitors (active);

CREATE TABLE IF NOT EXISTS check_logs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status_code INTEGER,
    latency_ms BIGINT,
    reachable BOOLEAN NOT NULL,
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_check_logs_monitor_checked_at
    ON check_logs (monitor_id, checked_at DESC);

CREATE TABLE IF NOT EXISTS incidents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL,
    recovered_at TIMESTAMPTZ,
    downtime_seconds BIGINT,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (recovered_at IS NULL OR recovered_at >= started_at),
    CHECK (downtime_seconds IS NULL OR downtime_seconds >= 0)
);

CREATE INDEX IF NOT EXISTS idx_incidents_monitor_started_at
    ON incidents (monitor_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_incidents_open
    ON incidents (monitor_id)
    WHERE recovered_at IS NULL;

CREATE TABLE IF NOT EXISTS daily_metrics (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    metric_date DATE NOT NULL,
    total_checks BIGINT NOT NULL DEFAULT 0 CHECK (total_checks >= 0),
    successful_checks BIGINT NOT NULL DEFAULT 0 CHECK (successful_checks >= 0),
    mean_latency_ms NUMERIC(12, 3),
    p95_latency_ms NUMERIC(12, 3),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (monitor_id, metric_date),
    CHECK (successful_checks <= total_checks)
);

CREATE INDEX IF NOT EXISTS idx_daily_metrics_monitor_date
    ON daily_metrics (monitor_id, metric_date DESC);
