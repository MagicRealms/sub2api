-- Catalog checks query the selected local group's model list without invoking
-- inference. Expand the legacy database guard as well as application validation.
ALTER TABLE channel_monitors
    DROP CONSTRAINT IF EXISTS channel_monitors_check_mode_check;
ALTER TABLE channel_monitors
    ADD CONSTRAINT channel_monitors_check_mode_check
    CHECK (check_mode IN ('probe', 'quota', 'quota_probe', 'catalog'));

COMMENT ON COLUMN channel_monitors.check_mode IS
    'probe: inference; quota: quota only; quota_probe: both; catalog: local model listing only, not inference health';
