-- Manual and scheduled IQ tests share one persisted running state. Keep the
-- execution deadline independent of later edits to the scheduled plan.
ALTER TABLE scheduled_test_results
    ADD COLUMN IF NOT EXISTS request_key VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS deadline_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_str_plan_request_key
    ON scheduled_test_results(plan_id, request_key)
    WHERE request_key <> '';

UPDATE scheduled_test_results AS result
SET deadline_at = COALESCE(plan.next_run_at, result.started_at + INTERVAL '30 minutes')
FROM scheduled_test_plans AS plan
WHERE result.plan_id = plan.id AND plan.kind = 'channel_iq'
  AND result.status = 'running' AND result.deadline_at IS NULL;
