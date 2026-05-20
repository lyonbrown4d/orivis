-- Ensure a monitor is owned by at most one agent.
DELETE FROM monitor_agents
WHERE rowid NOT IN (
    SELECT MIN(rowid)
    FROM monitor_agents
    GROUP BY monitor_id
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_monitor_agents_monitor_id
ON monitor_agents(monitor_id);
