package store

import (
	"context"
	"fmt"
	"time"
)

func (s *Store) sqlDashboardSnapshot(ctx context.Context, resultLimit int) (DashboardSnapshot, error) {
	agents, err := s.sqlDashboardAgents(ctx)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	monitors, err := s.sqlDashboardMonitors(ctx)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	results, err := s.sqlDashboardResults(ctx, resultLimit)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	return DashboardSnapshot{
		GeneratedAt: time.Now().UTC(),
		Agents:      agents,
		Monitors:    monitors,
		Results:     results,
	}, nil
}

func (s *Store) sqlDashboardAgents(ctx context.Context) ([]DashboardAgent, error) {
	rows, err := s.DB.QueryContext(
		ctx,
		`SELECT a.id, a.name, r.code, a.runtime_type, a.version, a.last_seen_at, a.status
         FROM agents a
         JOIN regions r ON r.id = a.region_id
         ORDER BY a.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list dashboard agents: %w", err)
	}
	defer closeRows(rows)

	agents := make([]DashboardAgent, 0)
	for rows.Next() {
		var agent DashboardAgent
		var lastSeenAt string
		if err := rows.Scan(
			&agent.ID,
			&agent.Name,
			&agent.RegionCode,
			&agent.RuntimeType,
			&agent.Version,
			&lastSeenAt,
			&agent.Status,
		); err != nil {
			return nil, fmt.Errorf("scan dashboard agent: %w", err)
		}
		parsedLastSeenAt, err := parseTime(lastSeenAt)
		if err != nil {
			return nil, err
		}
		agent.LastSeenAt = parsedLastSeenAt
		agent.EnvironmentCodes, err = s.sqlAgentEnvironmentCodes(ctx, agent.ID)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dashboard agents: %w", err)
	}
	return agents, nil
}

func (s *Store) sqlAgentEnvironmentCodes(ctx context.Context, agentID string) ([]string, error) {
	rows, err := s.DB.QueryContext(
		ctx,
		`SELECT e.code
         FROM agent_environments ae
         JOIN environments e ON e.id = ae.environment_id
         WHERE ae.agent_id = ?
         ORDER BY e.code`,
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list dashboard agent environments: %w", err)
	}
	defer closeRows(rows)

	codes := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, fmt.Errorf("scan dashboard agent environment: %w", err)
		}
		codes = append(codes, code)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dashboard agent environments: %w", err)
	}
	return codes, nil
}

func (s *Store) sqlDashboardMonitors(ctx context.Context) ([]DashboardMonitor, error) {
	rows, err := s.DB.QueryContext(
		ctx,
		`SELECT m.id, m.name, m.type, m.target, e.code, m.enabled,
                m.interval_seconds, m.timeout_seconds, m.retry_count,
                m.aggregation_policy, m.source
         FROM monitors m
         JOIN environments e ON e.id = m.environment_id
         ORDER BY e.code, m.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list dashboard monitors: %w", err)
	}
	defer closeRows(rows)

	monitors := make([]DashboardMonitor, 0)
	for rows.Next() {
		var monitor DashboardMonitor
		var enabled int
		var intervalSeconds, timeoutSeconds int
		if err := rows.Scan(
			&monitor.ID,
			&monitor.Name,
			&monitor.Type,
			&monitor.Target,
			&monitor.EnvironmentCode,
			&enabled,
			&intervalSeconds,
			&timeoutSeconds,
			&monitor.RetryCount,
			&monitor.AggregationPolicy,
			&monitor.Source,
		); err != nil {
			return nil, fmt.Errorf("scan dashboard monitor: %w", err)
		}
		monitor.Enabled = enabled == 1
		monitor.Interval = time.Duration(intervalSeconds) * time.Second
		monitor.Timeout = time.Duration(timeoutSeconds) * time.Second
		monitors = append(monitors, monitor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dashboard monitors: %w", err)
	}
	return monitors, nil
}

func (s *Store) sqlDashboardResults(ctx context.Context, limit int) ([]DashboardResult, error) {
	rows, err := s.DB.QueryContext(
		ctx,
		`SELECT pr.id, pr.monitor_id, pr.agent_id, a.name, r.code, e.code,
                pr.status, pr.latency_ms, pr.error_message, pr.checked_at, pr.created_at
         FROM probe_results pr
         JOIN agents a ON a.id = pr.agent_id
         JOIN regions r ON r.id = pr.region_id
         JOIN environments e ON e.id = pr.environment_id
         ORDER BY pr.checked_at DESC
         LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list dashboard results: %w", err)
	}
	defer closeRows(rows)

	results := make([]DashboardResult, 0)
	for rows.Next() {
		result, err := scanDashboardResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dashboard results: %w", err)
	}
	return results, nil
}

func scanDashboardResult(rows interface {
	Scan(dest ...any) error
}) (DashboardResult, error) {
	var result DashboardResult
	var latencyMS int64
	var checkedAt, createdAt string
	if err := rows.Scan(
		&result.ID,
		&result.MonitorID,
		&result.AgentID,
		&result.AgentName,
		&result.RegionCode,
		&result.EnvironmentCode,
		&result.Status,
		&latencyMS,
		&result.ErrorMessage,
		&checkedAt,
		&createdAt,
	); err != nil {
		return DashboardResult{}, fmt.Errorf("scan dashboard result: %w", err)
	}
	parsedCheckedAt, err := parseTime(checkedAt)
	if err != nil {
		return DashboardResult{}, err
	}
	parsedCreatedAt, err := parseTime(createdAt)
	if err != nil {
		return DashboardResult{}, err
	}
	result.Latency = time.Duration(latencyMS) * time.Millisecond
	result.CheckedAt = parsedCheckedAt
	result.CreatedAt = parsedCreatedAt
	return result, nil
}
