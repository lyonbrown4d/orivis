CREATE TABLE IF NOT EXISTS environments (
    id VARCHAR(255) PRIMARY KEY,
    name TEXT NOT NULL,
    code VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    source TEXT NOT NULL DEFAULT 'ui',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS regions (
    id VARCHAR(255) PRIMARY KEY,
    name TEXT NOT NULL,
    code VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    source TEXT NOT NULL DEFAULT 'ui',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS agents (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    token_hash TEXT NOT NULL,
    region_id VARCHAR(255) NOT NULL,
    runtime_type TEXT NOT NULL,
    version TEXT NOT NULL DEFAULT '',
    last_seen_at TEXT NOT NULL,
    status TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'api',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CONSTRAINT fk_agents_region_id FOREIGN KEY (region_id) REFERENCES regions(id)
);

CREATE INDEX idx_agents_region_id ON agents(region_id);
CREATE INDEX idx_agents_status ON agents(status(64));
CREATE INDEX idx_agents_last_seen_at ON agents(last_seen_at(64));

CREATE TABLE IF NOT EXISTS agent_environments (
    agent_id VARCHAR(255) NOT NULL,
    environment_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (agent_id, environment_id),
    CONSTRAINT fk_agent_environments_agent_id FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    CONSTRAINT fk_agent_environments_environment_id FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS monitors (
    id VARCHAR(255) PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    target TEXT NOT NULL,
    environment_id VARCHAR(255) NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    interval_seconds INTEGER NOT NULL,
    timeout_seconds INTEGER NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    aggregation_policy TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'ui',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CONSTRAINT fk_monitors_environment_id FOREIGN KEY (environment_id) REFERENCES environments(id)
);

CREATE INDEX idx_monitors_environment_id ON monitors(environment_id);
CREATE INDEX idx_monitors_enabled ON monitors(enabled);

CREATE TABLE IF NOT EXISTS monitor_agents (
    monitor_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    PRIMARY KEY (monitor_id, agent_id),
    CONSTRAINT fk_monitor_agents_monitor_id FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE,
    CONSTRAINT fk_monitor_agents_agent_id FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS probe_results (
    id VARCHAR(255) PRIMARY KEY,
    monitor_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    region_id VARCHAR(255) NOT NULL,
    environment_id VARCHAR(255) NOT NULL,
    status TEXT NOT NULL,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    checked_at TEXT NOT NULL,
    raw_detail LONGBLOB,
    created_at TEXT NOT NULL,
    CONSTRAINT fk_probe_results_monitor_id FOREIGN KEY (monitor_id) REFERENCES monitors(id),
    CONSTRAINT fk_probe_results_agent_id FOREIGN KEY (agent_id) REFERENCES agents(id),
    CONSTRAINT fk_probe_results_region_id FOREIGN KEY (region_id) REFERENCES regions(id),
    CONSTRAINT fk_probe_results_environment_id FOREIGN KEY (environment_id) REFERENCES environments(id)
);

CREATE INDEX idx_probe_results_monitor_checked ON probe_results(monitor_id, checked_at(64));
CREATE INDEX idx_probe_results_agent_checked ON probe_results(agent_id, checked_at(64));
