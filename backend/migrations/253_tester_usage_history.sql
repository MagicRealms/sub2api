-- Testers reset only their personal observation window. Never delete usage,
-- billing deduplication, quota counters, or administrator audit history.
-- Additive so the preceding application image remains a valid rollback.
CREATE TABLE IF NOT EXISTS tester_usage_history (
    user_id bigint PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    visible_after timestamptz NOT NULL
);
