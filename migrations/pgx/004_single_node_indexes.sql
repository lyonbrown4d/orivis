CREATE INDEX IF NOT EXISTS idx_monitor_agents_agent_id ON monitor_agents(agent_id);
CREATE INDEX IF NOT EXISTS idx_probe_results_checked_at ON probe_results(checked_at);
CREATE INDEX IF NOT EXISTS idx_probe_results_monitor_status_checked ON probe_results(monitor_id, status, checked_at);
CREATE INDEX IF NOT EXISTS idx_probe_results_environment_status_checked ON probe_results(environment_id, status, checked_at);
