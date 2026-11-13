-- One optional active V2 probe. The probe is deliberately a singleton on the
-- V2 config so its claim and status are atomic across application instances.
ALTER TABLE channel_monitor_v2_config
    ADD COLUMN IF NOT EXISTS probe_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS probe_api_key_id BIGINT,
    ADD COLUMN IF NOT EXISTS probe_group_id BIGINT,
    ADD COLUMN IF NOT EXISTS probe_model TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS probe_next_run_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS probe_last_checked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS probe_status TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS probe_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS probe_message TEXT NOT NULL DEFAULT '';

ALTER TABLE channel_monitor_v2_config
    DROP CONSTRAINT IF EXISTS channel_monitor_v2_probe_pair,
    ADD CONSTRAINT channel_monitor_v2_probe_pair CHECK (
        (probe_enabled = FALSE)
        OR (probe_enabled = TRUE AND probe_api_key_id IS NOT NULL AND probe_api_key_id > 0
            AND probe_group_id IS NOT NULL AND probe_group_id > 0 AND NULLIF(TRIM(probe_model), '') IS NOT NULL)
    );

COMMENT ON COLUMN channel_monitor_v2_config.probe_next_run_at IS
    'The next hourly probe deadline; claimed atomically and advanced before the request starts';
