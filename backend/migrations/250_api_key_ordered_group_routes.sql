-- Additive migration: older application images continue to use group_id.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS group_ids JSONB;
COMMENT ON COLUMN api_keys.group_ids IS 'Ordered group routes; null preserves legacy single-group behavior';
