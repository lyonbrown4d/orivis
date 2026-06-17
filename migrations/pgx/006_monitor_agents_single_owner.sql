DELETE FROM monitor_agents
WHERE NOT EXISTS (
    SELECT 1
    FROM (
        SELECT monitor_id, MIN(agent_id) AS keep_agent_id
        FROM monitor_agents
        GROUP BY monitor_id
    ) keepers
    WHERE keepers.monitor_id = monitor_agents.monitor_id
      AND keepers.keep_agent_id = monitor_agents.agent_id
);

CREATE UNIQUE INDEX idx_monitor_agents_monitor_id ON monitor_agents(monitor_id);
