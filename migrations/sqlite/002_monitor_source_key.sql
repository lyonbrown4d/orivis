ALTER TABLE monitors ADD COLUMN source_key TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_monitors_source_key ON monitors(source_key) WHERE source_key <> '';
