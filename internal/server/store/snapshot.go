package store

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/lyonbrown4d/orivis/internal/shared/model"
)

type DashboardSnapshot struct {
	GeneratedAt time.Time
	Agents      []DashboardAgent
	Monitors    []DashboardMonitor
	Results     []DashboardResult
}

type DashboardAgent struct {
	ID               string
	Name             string
	RegionCode       string
	EnvironmentCodes []string
	RuntimeType      string
	Version          string
	LastSeenAt       time.Time
	Status           model.AgentStatus
}

type DashboardMonitor struct {
	ID                string
	Name              string
	Type              model.MonitorType
	Target            string
	EnvironmentCode   string
	Enabled           bool
	Interval          time.Duration
	Timeout           time.Duration
	RetryCount        int
	AggregationPolicy model.AggregationPolicy
	Source            model.ConfigSource
}

type DashboardResult struct {
	ID              string
	MonitorID       string
	AgentID         string
	AgentName       string
	RegionCode      string
	EnvironmentCode string
	Status          model.Status
	Latency         time.Duration
	ErrorMessage    string
	CheckedAt       time.Time
	CreatedAt       time.Time
}

func (s *Store) DashboardSnapshot(ctx context.Context, resultLimit int) (DashboardSnapshot, error) {
	if resultLimit <= 0 {
		resultLimit = 50
	}

	out := DashboardSnapshot{GeneratedAt: time.Now().UTC()}
	switch {
	case s == nil:
		return out, nil
	case s.memory != nil:
		return s.memoryDashboardSnapshot(resultLimit)
	case s.DB != nil:
		return s.sqlDashboardSnapshot(ctx, resultLimit)
	default:
		return out, nil
	}
}

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
	defer rows.Close()

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
	defer rows.Close()

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
	defer rows.Close()

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
	defer rows.Close()

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

func (s *Store) memoryDashboardSnapshot(resultLimit int) (DashboardSnapshot, error) {
	txn := s.memory.db.Txn(false)
	defer txn.Abort()

	agents, err := memoryDashboardAgents(txn)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	monitors, err := memoryDashboardMonitors(txn)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	results, err := memoryDashboardResults(txn, resultLimit)
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

func memoryDashboardAgents(txn *memdb.Txn) ([]DashboardAgent, error) {
	regions, err := memoryRegionCodes(txn)
	if err != nil {
		return nil, err
	}
	environments, err := memoryEnvironmentCodes(txn)
	if err != nil {
		return nil, err
	}
	it, err := txn.Get(memoryTableAgents, "id")
	if err != nil {
		return nil, fmt.Errorf("list memory dashboard agents: %w", err)
	}

	agents := make([]DashboardAgent, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		agent := raw.(*model.Agent)
		environmentIDs, err := memoryAgentEnvironmentIDs(txn, agent.ID)
		if err != nil {
			return nil, err
		}
		agents = append(agents, DashboardAgent{
			ID:               agent.ID,
			Name:             agent.Name,
			RegionCode:       regions[agent.RegionID],
			EnvironmentCodes: memoryCodesForIDs(environmentIDs, environments),
			RuntimeType:      agent.RuntimeType,
			Version:          agent.Version,
			LastSeenAt:       agent.LastSeenAt,
			Status:           agent.Status,
		})
	}
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Name < agents[j].Name
	})
	return agents, nil
}

func memoryDashboardMonitors(txn *memdb.Txn) ([]DashboardMonitor, error) {
	environments, err := memoryEnvironmentCodes(txn)
	if err != nil {
		return nil, err
	}
	it, err := txn.Get(memoryTableMonitors, "id")
	if err != nil {
		return nil, fmt.Errorf("list memory dashboard monitors: %w", err)
	}

	monitors := make([]DashboardMonitor, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		monitor := raw.(*model.Monitor)
		monitors = append(monitors, DashboardMonitor{
			ID:                monitor.ID,
			Name:              monitor.Name,
			Type:              monitor.Type,
			Target:            monitor.Target,
			EnvironmentCode:   environments[monitor.EnvironmentID],
			Enabled:           monitor.Enabled,
			Interval:          monitor.Interval,
			Timeout:           monitor.Timeout,
			RetryCount:        monitor.RetryCount,
			AggregationPolicy: monitor.AggregationPolicy,
			Source:            monitor.Source,
		})
	}
	sort.Slice(monitors, func(i, j int) bool {
		if monitors[i].EnvironmentCode == monitors[j].EnvironmentCode {
			return monitors[i].Name < monitors[j].Name
		}
		return monitors[i].EnvironmentCode < monitors[j].EnvironmentCode
	})
	return monitors, nil
}

func memoryDashboardResults(txn *memdb.Txn, limit int) ([]DashboardResult, error) {
	regions, err := memoryRegionCodes(txn)
	if err != nil {
		return nil, err
	}
	environments, err := memoryEnvironmentCodes(txn)
	if err != nil {
		return nil, err
	}
	agents, err := memoryAgentNames(txn)
	if err != nil {
		return nil, err
	}

	it, err := txn.Get(memoryTableProbeResults, "id")
	if err != nil {
		return nil, fmt.Errorf("list memory dashboard results: %w", err)
	}

	results := make([]DashboardResult, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		result := raw.(*model.ProbeResult)
		results = append(results, DashboardResult{
			ID:              result.ID,
			MonitorID:       result.MonitorID,
			AgentID:         result.AgentID,
			AgentName:       agents[result.AgentID],
			RegionCode:      regions[result.RegionID],
			EnvironmentCode: environments[result.EnvironmentID],
			Status:          result.Status,
			Latency:         result.Latency,
			ErrorMessage:    result.ErrorMessage,
			CheckedAt:       result.CheckedAt,
			CreatedAt:       result.CreatedAt,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CheckedAt.After(results[j].CheckedAt)
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func memoryRegionCodes(txn *memdb.Txn) (map[string]string, error) {
	it, err := txn.Get(memoryTableRegions, "id")
	if err != nil {
		return nil, fmt.Errorf("list memory regions: %w", err)
	}
	codes := map[string]string{}
	for raw := it.Next(); raw != nil; raw = it.Next() {
		region := raw.(*model.Region)
		codes[region.ID] = region.Code
	}
	return codes, nil
}

func memoryEnvironmentCodes(txn *memdb.Txn) (map[string]string, error) {
	it, err := txn.Get(memoryTableEnvironments, "id")
	if err != nil {
		return nil, fmt.Errorf("list memory environments: %w", err)
	}
	codes := map[string]string{}
	for raw := it.Next(); raw != nil; raw = it.Next() {
		environment := raw.(*model.Environment)
		codes[environment.ID] = environment.Code
	}
	return codes, nil
}

func memoryAgentNames(txn *memdb.Txn) (map[string]string, error) {
	it, err := txn.Get(memoryTableAgents, "id")
	if err != nil {
		return nil, fmt.Errorf("list memory agents: %w", err)
	}
	names := map[string]string{}
	for raw := it.Next(); raw != nil; raw = it.Next() {
		agent := raw.(*model.Agent)
		names[agent.ID] = agent.Name
	}
	return names, nil
}

func memoryCodesForIDs(ids []string, codes map[string]string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if code := strings.TrimSpace(codes[id]); code != "" {
			out = append(out, code)
		}
	}
	sort.Strings(out)
	return out
}
