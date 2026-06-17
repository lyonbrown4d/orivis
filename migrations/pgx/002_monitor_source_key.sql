ALTER TABLE monitors ADD COLUMN source_key TEXT;

UPDATE monitors
SET source_key = NULL
WHERE source_key = '';

CREATE UNIQUE INDEX idx_monitors_source_key ON monitors(source_key);
