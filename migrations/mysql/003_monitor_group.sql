ALTER TABLE monitors ADD COLUMN group_name TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_monitors_group_name ON monitors(group_name(191));
