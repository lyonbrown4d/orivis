package store

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-memdb"
	"github.com/lyonbrown4d/orivis/internal/model"
)

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
		agent, err := memoryValue[model.Agent](raw, "agent")
		if err != nil {
			return nil, err
		}
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
		monitor, err := memoryValue[model.Monitor](raw, "monitor")
		if err != nil {
			return nil, err
		}
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
		result, err := memoryValue[model.ProbeResult](raw, "probe result")
		if err != nil {
			return nil, err
		}
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
		region, err := memoryValue[model.Region](raw, "region")
		if err != nil {
			return nil, err
		}
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
		environment, err := memoryValue[model.Environment](raw, "environment")
		if err != nil {
			return nil, err
		}
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
		agent, err := memoryValue[model.Agent](raw, "agent")
		if err != nil {
			return nil, err
		}
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
