CREATE INDEX idx_monitor_agents_agent_id ON monitor_agents(agent_id);
CREATE INDEX idx_probe_results_checked_at ON probe_results(checked_at(64));
CREATE INDEX idx_probe_results_monitor_status_checked ON probe_results(monitor_id, status(64), checked_at(64));
CREATE INDEX idx_probe_results_environment_status_checked ON probe_results(environment_id, status(64), checked_at(64));
