ALTER TABLE monitors ADD COLUMN group_name TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_monitors_group_name ON monitors(group_name);
