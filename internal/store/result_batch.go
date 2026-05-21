package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionset "github.com/arcgolabs/collectionx/set"
	"github.com/lyonbrown4d/orivis/internal/model"
)

func normalizeProbeResultParamList(params []RecordProbeResultParams) (*collectionlist.List[normalizedProbeResultParams], error) {
	out := collectionlist.NewListWithCapacity[normalizedProbeResultParams](len(params))
	for index := range params {
		normalized, err := normalizeProbeResultParams(params[index])
		if err != nil {
			return nil, err
		}
		out.Add(normalized)
	}
	return out, nil
}

func (s *resultStore) monitorLookupForAgentBatch(
	ctx context.Context,
	queryer resultQueryer,
	params []normalizedProbeResultParams,
) (monitors map[string]model.Monitor, err error) {
	query, args := monitorForAgentBatchQuery(params)
	if query == "" {
		return map[string]model.Monitor{}, nil
	}
	rows, err := queryer.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find monitors for agents: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close monitors for agents rows: %w", closeErr)
		}
	}()
	monitors, err = scanMonitorForAgentRows(rows)
	if err != nil {
		return nil, err
	}
	return monitors, nil
}

func monitorForAgentBatchQuery(params []normalizedProbeResultParams) (string, []any) {
	pairs := uniqueMonitorAgentPairs(params)
	if len(pairs) == 0 {
		return "", nil
	}
	conditions := make([]string, len(pairs))
	args := make([]any, 0, len(pairs)*2)
	for index, pair := range pairs {
		conditions[index] = "(m.id = ? AND ma.agent_id = ?)"
		args = append(args, pair.monitorID, pair.agentID)
	}
	return `SELECT m.id, m.name, m.type, m.target, m.environment_id, m.enabled,
                m.interval_seconds, m.timeout_seconds, m.retry_count,
                m.aggregation_policy, m.source, m.created_at, m.updated_at,
                ma.agent_id
         FROM monitors m
         JOIN monitor_agents ma ON ma.monitor_id = m.id
         WHERE m.enabled = 1 AND (` + strings.Join(conditions, " OR ") + `)`, args
}

type monitorAgentPair struct {
	monitorID string
	agentID   string
}

func uniqueMonitorAgentPairs(params []normalizedProbeResultParams) []monitorAgentPair {
	seen := collectionset.NewSetWithCapacity[string](len(params))
	return collectionlist.FilterMapList(collectionlist.NewList(params...), func(_ int, params normalizedProbeResultParams) (monitorAgentPair, bool) {
		key := monitorAgentKey(params.MonitorID, params.Agent.ID)
		if seen.Contains(key) {
			return monitorAgentPair{}, false
		}
		seen.Add(key)
		return monitorAgentPair{monitorID: params.MonitorID, agentID: params.Agent.ID}, true
	}).Values()
}

func scanMonitorForAgentRows(rows *sql.Rows) (map[string]model.Monitor, error) {
	monitors := make(map[string]model.Monitor)
	for rows.Next() {
		var rec monitorRecord
		var agentID string
		if err := rows.Scan(
			&rec.ID,
			&rec.Name,
			&rec.Type,
			&rec.Target,
			&rec.EnvironmentID,
			&rec.Enabled,
			&rec.IntervalSeconds,
			&rec.TimeoutSeconds,
			&rec.RetryCount,
			&rec.AggregationPolicy,
			&rec.Source,
			&rec.CreatedAt,
			&rec.UpdatedAt,
			&agentID,
		); err != nil {
			return nil, fmt.Errorf("scan monitor for agent: %w", err)
		}
		monitor, err := rec.model()
		if err != nil {
			return nil, err
		}
		monitors[monitorAgentKey(rec.ID, agentID)] = monitor
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monitors for agents: %w", err)
	}
	return monitors, nil
}

func monitorAgentKey(monitorID, agentID string) string {
	return monitorID + "\x00" + agentID
}
