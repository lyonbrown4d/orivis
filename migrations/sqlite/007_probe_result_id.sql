ALTER TABLE probe_results ADD COLUMN result_id TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_probe_results_result_id ON probe_results(result_id) WHERE result_id <> '';
