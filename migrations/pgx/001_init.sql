CREATE TABLE IF NOT EXISTS environments (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    source TEXT NOT NULL DEFAULT 'ui',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS regions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    code TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    source TEXT NOT NULL DEFAULT 'ui',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    token_hash TEXT NOT NULL,
    region_id TEXT NOT NULL,
    runtime_type TEXT NOT NULL,
    version TEXT NOT NULL DEFAULT '',
    last_seen_at TEXT NOT NULL,
    status TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'api',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (region_id) REFERENCES regions(id)
);

CREATE INDEX IF NOT EXISTS idx_agents_region_id ON agents(region_id);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_last_seen_at ON agents(last_seen_at);

CREATE TABLE IF NOT EXISTS agent_environments (
    agent_id TEXT NOT NULL,
    environment_id TEXT NOT NULL,
    PRIMARY KEY (agent_id, environment_id),
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS monitors (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    target TEXT NOT NULL,
    environment_id TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    interval_seconds INTEGER NOT NULL,
    timeout_seconds INTEGER NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    aggregation_policy TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'ui',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    FOREIGN KEY (environment_id) REFERENCES environments(id)
);

CREATE INDEX IF NOT EXISTS idx_monitors_environment_id ON monitors(environment_id);
CREATE INDEX IF NOT EXISTS idx_monitors_enabled ON monitors(enabled);

CREATE TABLE IF NOT EXISTS monitor_agents (
    monitor_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    PRIMARY KEY (monitor_id, agent_id),
    FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS probe_results (
    id TEXT PRIMARY KEY,
    monitor_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    region_id TEXT NOT NULL,
    environment_id TEXT NOT NULL,
    status TEXT NOT NULL,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    checked_at TEXT NOT NULL,
    raw_detail BYTEA,
    created_at TEXT NOT NULL,
    FOREIGN KEY (monitor_id) REFERENCES monitors(id),
    FOREIGN KEY (agent_id) REFERENCES agents(id),
    FOREIGN KEY (region_id) REFERENCES regions(id),
    FOREIGN KEY (environment_id) REFERENCES environments(id)
);

CREATE INDEX IF NOT EXISTS idx_probe_results_monitor_checked ON probe_results(monitor_id, checked_at);
CREATE INDEX IF NOT EXISTS idx_probe_results_agent_checked ON probe_results(agent_id, checked_at);
