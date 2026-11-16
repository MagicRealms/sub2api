ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS strict_priority_enabled BOOLEAN;

COMMENT ON COLUMN groups.strict_priority_enabled IS
    'Per-group OpenAI text strict account priority override; NULL uses the legacy configured group ID list';
