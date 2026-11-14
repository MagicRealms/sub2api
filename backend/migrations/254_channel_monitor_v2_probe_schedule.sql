-- Make the V2 probe cadence and real-traffic pause operator-configurable.
-- The service validates the supported choices; these checks only protect the
-- scheduler from malformed values written outside the API.
ALTER TABLE channel_monitor_v2_config
    ADD COLUMN IF NOT EXISTS probe_interval_seconds INTEGER NOT NULL DEFAULT 3600,
    ADD COLUMN IF NOT EXISTS probe_pause_after_traffic_seconds INTEGER NOT NULL DEFAULT 10800,
    ADD COLUMN IF NOT EXISTS probe_suppressed_until TIMESTAMPTZ;

ALTER TABLE channel_monitor_v2_config
    DROP CONSTRAINT IF EXISTS channel_monitor_v2_probe_schedule_bounds,
    ADD CONSTRAINT channel_monitor_v2_probe_schedule_bounds CHECK (
        probe_interval_seconds BETWEEN 1800 AND 18000
        AND probe_pause_after_traffic_seconds BETWEEN 3600 AND 43200
    );

