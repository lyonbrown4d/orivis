package store

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/arcgolabs/dbx/querydsl"
	"github.com/lyonbrown4d/orivis/internal/model"
)

func (s *Store) sqlDashboardSnapshot(ctx context.Context, resultLimit int) (DashboardSnapshot, error) {
	regions, err := s.sqlDashboardRegions(ctx)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	environments, err := s.sqlDashboardEnvironments(ctx)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	agentEnvironments, err := s.sqlDashboardAgentEnvironments(ctx, environments)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	agents, err := s.sqlDashboardAgents(ctx, regions, agentEnvironments)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	monitors, err := s.sqlDashboardMonitors(ctx, environments)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	results, err := s.sqlDashboardResults(ctx, resultLimit, agents, regions, environments, monitors)
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

func (s *Store) sqlDashboardRegions(ctx context.Context) (map[string]string, error) {
	rows, err := s.repositories.regions.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(regionsSchema).Values()...).
			From(regionsSchema),
	)
	if err != nil {
		return nil, fmt.Errorf("list dashboard regions: %w", err)
	}
	out := make(map[string]string, rows.Len())
	regionRows := rows.Values()
	for index := range regionRows {
		out[regionRows[index].ID] = regionRows[index].Code
	}
	return out, nil
}

func (s *Store) sqlDashboardEnvironments(ctx context.Context) (map[string]string, error) {
	rows, err := s.repositories.environments.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(environmentsSchema).Values()...).
			From(environmentsSchema),
	)
	if err != nil {
		return nil, fmt.Errorf("list dashboard environments: %w", err)
	}
	out := make(map[string]string, rows.Len())
	environmentRows := rows.Values()
	for index := range environmentRows {
		out[environmentRows[index].ID] = environmentRows[index].Code
	}
	return out, nil
}

func (s *Store) sqlDashboardAgentEnvironments(ctx context.Context, environments map[string]string) (map[string][]string, error) {
	rows, err := s.repositories.agentEnvironments.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(agentEnvironmentsSchema).Values()...).
			From(agentEnvironmentsSchema).
			OrderBy(agentEnvironmentsSchema.AgentID.Asc(), agentEnvironmentsSchema.EnvironmentID.Asc()),
	)
	if err != nil {
		return nil, fmt.Errorf("list dashboard agent environments: %w", err)
	}
	out := make(map[string][]string)
	agentEnvironmentRows := rows.Values()
	for index := range agentEnvironmentRows {
		code := environments[agentEnvironmentRows[index].EnvironmentID]
		if code == "" {
			continue
		}
		out[agentEnvironmentRows[index].AgentID] = append(out[agentEnvironmentRows[index].AgentID], code)
	}
	for agentID := range out {
		slices.Sort(out[agentID])
	}
	return out, nil
}

func (s *Store) sqlDashboardAgents(
	ctx context.Context,
	regions map[string]string,
	agentEnvironments map[string][]string,
) ([]DashboardAgent, error) {
	rows, err := s.repositories.agents.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(agentsSchema).Values()...).
			From(agentsSchema).
			OrderBy(agentsSchema.Name.Asc()),
	)
	if err != nil {
		return nil, fmt.Errorf("list dashboard agents: %w", err)
	}
	agents := make([]DashboardAgent, 0, rows.Len())
	agentRows := rows.Values()
	for index := range agentRows {
		lastSeenAt, err := parseTime(agentRows[index].LastSeenAt)
		if err != nil {
			return nil, err
		}
		agents = append(agents, DashboardAgent{
			ID:               agentRows[index].ID,
			Name:             agentRows[index].Name,
			RegionCode:       regions[agentRows[index].RegionID],
			EnvironmentCodes: append([]string(nil), agentEnvironments[agentRows[index].ID]...),
			RuntimeType:      agentRows[index].RuntimeType,
			Version:          agentRows[index].Version,
			LastSeenAt:       lastSeenAt,
			Status:           model.AgentStatus(agentRows[index].Status),
		})
	}
	return agents, nil
}

func (s *Store) sqlDashboardMonitors(ctx context.Context, environments map[string]string) ([]DashboardMonitor, error) {
	rows, err := s.repositories.monitors.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(monitorsSchema).Values()...).
			From(monitorsSchema),
	)
	if err != nil {
		return nil, fmt.Errorf("list dashboard monitors: %w", err)
	}
	monitors := make([]DashboardMonitor, 0, rows.Len())
	monitorRows := rows.Values()
	for index := range monitorRows {
		monitors = append(monitors, DashboardMonitor{
			ID:                monitorRows[index].ID,
			SourceKey:         monitorRows[index].SourceKey,
			Name:              monitorRows[index].Name,
			Type:              model.MonitorType(monitorRows[index].Type),
			Target:            monitorRows[index].Target,
			GroupName:         monitorRows[index].GroupName,
			EnvironmentCode:   environments[monitorRows[index].EnvironmentID],
			Enabled:           monitorRows[index].Enabled == 1,
			Interval:          time.Duration(monitorRows[index].IntervalSeconds) * time.Second,
			Timeout:           time.Duration(monitorRows[index].TimeoutSeconds) * time.Second,
			RetryCount:        monitorRows[index].RetryCount,
			AggregationPolicy: model.AggregationPolicy(monitorRows[index].AggregationPolicy),
			Source:            model.ConfigSource(monitorRows[index].Source),
		})
	}
	slices.SortFunc(monitors, func(left, right DashboardMonitor) int {
		if left.EnvironmentCode != right.EnvironmentCode {
			return cmpString(left.EnvironmentCode, right.EnvironmentCode)
		}
		return cmpString(left.Name, right.Name)
	})
	return monitors, nil
}

func (s *Store) sqlDashboardResults(
	ctx context.Context,
	limit int,
	agents []DashboardAgent,
	regions map[string]string,
	environments map[string]string,
	monitors []DashboardMonitor,
) ([]DashboardResult, error) {
	rows, err := s.repositories.probeResults.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(probeResultSchemaResource()).Values()...).
			From(probeResultSchemaResource()).
			OrderBy(probeResultSchemaResource().CheckedAt.Desc()).
			Limit(limit),
	)
	if err != nil {
		return nil, fmt.Errorf("list dashboard results: %w", err)
	}
	agentNames := dashboardAgentNames(agents)
	monitorGroups := dashboardMonitorGroups(monitors)
	results := make([]DashboardResult, 0, rows.Len())
	resultRows := rows.Values()
	for index := range resultRows {
		checkedAt, err := parseTime(resultRows[index].CheckedAt)
		if err != nil {
			return nil, err
		}
		createdAt, err := parseTime(resultRows[index].CreatedAt)
		if err != nil {
			return nil, err
		}
		results = append(results, DashboardResult{
			ID:              resultRows[index].ID,
			MonitorID:       resultRows[index].MonitorID,
			AgentID:         resultRows[index].AgentID,
			AgentName:       agentNames[resultRows[index].AgentID],
			RegionCode:      regions[resultRows[index].RegionID],
			EnvironmentCode: environments[resultRows[index].EnvironmentID],
			GroupName:       monitorGroups[resultRows[index].MonitorID],
			Status:          model.Status(resultRows[index].Status),
			Latency:         time.Duration(resultRows[index].LatencyMS) * time.Millisecond,
			ErrorMessage:    resultRows[index].ErrorMessage,
			CheckedAt:       checkedAt,
			CreatedAt:       createdAt,
		})
	}
	return results, nil
}

func dashboardAgentNames(agents []DashboardAgent) map[string]string {
	out := make(map[string]string, len(agents))
	for index := range agents {
		out[agents[index].ID] = agents[index].Name
	}
	return out
}

func dashboardMonitorGroups(monitors []DashboardMonitor) map[string]string {
	out := make(map[string]string, len(monitors))
	for index := range monitors {
		out[monitors[index].ID] = monitors[index].GroupName
	}
	return out
}

func cmpString(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
