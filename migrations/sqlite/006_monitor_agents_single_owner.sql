-- Keep exactly one owner per monitor, using lexicographically smallest agent id.
DELETE FROM monitor_agents
WHERE (monitor_id, agent_id) NOT IN (
    SELECT monitor_id, MIN(agent_id)
    FROM monitor_agents
    GROUP BY monitor_id
);

CREATE UNIQUE INDEX idx_monitor_agents_monitor_id
ON monitor_agents(monitor_id);
