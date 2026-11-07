-- Additive, rolling-upgrade compatible. Historical usage and monetary counters
-- remain immutable; these timestamps affect only the owner's key-list display.
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS usage_today_reset_at timestamptz,
    ADD COLUMN IF NOT EXISTS usage_total_reset_at timestamptz;

-- Keep references when a key/group is soft-deleted so a scheduled probe fails
-- closed instead of silently reverting to another group or an external URL.
ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS local_api_key_id bigint,
    ADD COLUMN IF NOT EXISTS probe_group_id bigint;
ALTER TABLE channel_monitors ADD CONSTRAINT channel_monitor_local_probe_pair
    CHECK ((local_api_key_id IS NULL AND probe_group_id IS NULL)
        OR (local_api_key_id IS NOT NULL AND probe_group_id IS NOT NULL AND local_api_key_id > 0 AND probe_group_id > 0));
