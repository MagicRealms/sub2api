ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS exclude_from_ops BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN groups.exclude_from_ops IS
    'Exclude this group from Ops summaries unless explicitly selected';
