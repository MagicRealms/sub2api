-- Keep one low-token probe target per public group and retain its latest
-- observations so the V2 channel status strip can show real probe results.
ALTER TABLE channel_monitor_v2_config
    DROP CONSTRAINT IF EXISTS channel_monitor_v2_probe_pair;

CREATE TABLE IF NOT EXISTS channel_monitor_v2_probe_targets (
    id BIGSERIAL PRIMARY KEY,
    api_key_id BIGINT NOT NULL CHECK (api_key_id > 0),
    group_id BIGINT NOT NULL UNIQUE CHECK (group_id > 0),
    model TEXT NOT NULL CHECK (NULLIF(TRIM(model), '') IS NOT NULL),
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    suppressed_until TIMESTAMPTZ,
    last_checked_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT '',
    latency_ms BIGINT,
    message TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_channel_monitor_v2_probe_targets_due
    ON channel_monitor_v2_probe_targets (next_run_at, id);

CREATE TABLE IF NOT EXISTS channel_monitor_v2_probe_history (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL,
    model TEXT NOT NULL,
    checked_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL,
    latency_ms BIGINT
);

CREATE INDEX IF NOT EXISTS idx_channel_monitor_v2_probe_history_group_time
    ON channel_monitor_v2_probe_history (group_id, checked_at DESC, id DESC);

INSERT INTO channel_monitor_v2_probe_targets (
    api_key_id, group_id, model, next_run_at, suppressed_until,
    last_checked_at, status, latency_ms, message
)
SELECT probe_api_key_id, probe_group_id, probe_model,
       COALESCE(probe_next_run_at, NOW()), probe_suppressed_until,
       probe_last_checked_at, probe_status, probe_latency_ms, COALESCE(probe_message, '')
FROM channel_monitor_v2_config
WHERE id = 1 AND probe_enabled = TRUE
  AND probe_api_key_id IS NOT NULL AND probe_group_id IS NOT NULL
  AND NULLIF(TRIM(probe_model), '') IS NOT NULL
ON CONFLICT (group_id) DO NOTHING;

INSERT INTO channel_monitor_v2_probe_history (group_id, model, checked_at, status, latency_ms)
SELECT probe_group_id, probe_model, probe_last_checked_at,
       CASE WHEN probe_status = 'paused_traffic' THEN 'operational' ELSE probe_status END,
       probe_latency_ms
FROM channel_monitor_v2_config
WHERE id = 1 AND probe_last_checked_at IS NOT NULL
  AND probe_api_key_id IS NOT NULL AND probe_group_id IS NOT NULL
  AND probe_status IN ('operational', 'degraded', 'error')
  AND NULLIF(TRIM(probe_model), '') IS NOT NULL;
