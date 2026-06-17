DELETE ma
FROM monitor_agents ma
LEFT JOIN (
    SELECT monitor_id, MIN(agent_id) AS keep_agent_id
    FROM monitor_agents
    GROUP BY monitor_id
) keepers
ON keepers.monitor_id = ma.monitor_id AND keepers.keep_agent_id = ma.agent_id
WHERE keepers.keep_agent_id IS NULL;

CREATE UNIQUE INDEX idx_monitor_agents_monitor_id ON monitor_agents(monitor_id);
