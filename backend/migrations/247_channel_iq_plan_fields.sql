-- Dedicated persisted channel intelligence plan fields.
-- Existing scheduled account tests keep kind='account'; Channel IQ owns one
-- plan with kind='channel_iq' and stores its prompt/effort beside the plan.

ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS kind VARCHAR(32) NOT NULL DEFAULT 'account';

ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS prompt TEXT NOT NULL DEFAULT '';

ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS reasoning_effort VARCHAR(32) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_stp_kind_enabled_next_run
    ON scheduled_test_plans(kind, enabled, next_run_at)
    WHERE enabled = true;

CREATE UNIQUE INDEX IF NOT EXISTS idx_stp_single_channel_iq
    ON scheduled_test_plans(kind)
    WHERE kind = 'channel_iq';

ALTER TABLE scheduled_test_results
    ADD COLUMN IF NOT EXISTS model_id VARCHAR(200) NOT NULL DEFAULT '';

ALTER TABLE scheduled_test_results
    ADD COLUMN IF NOT EXISTS reasoning_effort VARCHAR(32) NOT NULL DEFAULT '';
