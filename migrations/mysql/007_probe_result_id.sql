ALTER TABLE probe_results ADD COLUMN result_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE probe_results ADD COLUMN result_id_unique VARCHAR(255) GENERATED ALWAYS AS (NULLIF(result_id, '')) STORED;
CREATE UNIQUE INDEX idx_probe_results_result_id ON probe_results(result_id_unique);
