package store

import (
	"context"
	"fmt"
	"slices"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
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
	return collectionlist.ReduceList(rows, make(map[string]string, rows.Len()), func(out map[string]string, _ int, row regionRow) map[string]string {
		out[row.ID] = row.Code
		return out
	}), nil
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
	return collectionlist.ReduceList(rows, make(map[string]string, rows.Len()), func(out map[string]string, _ int, row environmentRow) map[string]string {
		out[row.ID] = row.Code
		return out
	}), nil
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
	out := collectionlist.ReduceList(rows, map[string][]string{}, func(out map[string][]string, _ int, row agentEnvironmentRow) map[string][]string {
		code := environments[row.EnvironmentID]
		if code == "" {
			return out
		}
		out[row.AgentID] = append(out[row.AgentID], code)
		return out
	})
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
	agents, err := collectionlist.ReduceErrList(
		rows,
		collectionlist.NewListWithCapacity[DashboardAgent](rows.Len()),
		func(out *collectionlist.List[DashboardAgent], _ int, row agentRecord) (*collectionlist.List[DashboardAgent], error) {
			lastSeenAt, parseErr := parseTime(row.LastSeenAt)
			if parseErr != nil {
				return nil, parseErr
			}
			out.Add(DashboardAgent{
				ID:               row.ID,
				Name:             row.Name,
				RegionCode:       regions[row.RegionID],
				EnvironmentCodes: append([]string(nil), agentEnvironments[row.ID]...),
				RuntimeType:      row.RuntimeType,
				Version:          row.Version,
				LastSeenAt:       lastSeenAt,
				Status:           model.AgentStatus(row.Status),
			})
			return out, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("build dashboard agents: %w", err)
	}
	return agents.Values(), nil
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
	monitors := collectionlist.MapList(rows, func(_ int, row monitorRecord) DashboardMonitor {
		return DashboardMonitor{
			ID:                row.ID,
			SourceKey:         row.SourceKey,
			Name:              row.Name,
			Type:              model.MonitorType(row.Type),
			Target:            row.Target,
			GroupName:         row.GroupName,
			EnvironmentCode:   environments[row.EnvironmentID],
			Enabled:           row.Enabled == 1,
			Interval:          time.Duration(row.IntervalSeconds) * time.Second,
			Timeout:           time.Duration(row.TimeoutSeconds) * time.Second,
			RetryCount:        row.RetryCount,
			AggregationPolicy: model.AggregationPolicy(row.AggregationPolicy),
			Source:            model.ConfigSource(row.Source),
		}
	}).Values()
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
	results, err := collectionlist.ReduceErrList(
		rows,
		collectionlist.NewListWithCapacity[DashboardResult](rows.Len()),
		func(out *collectionlist.List[DashboardResult], _ int, row probeResultRow) (*collectionlist.List[DashboardResult], error) {
			checkedAt, parseErr := parseTime(row.CheckedAt)
			if parseErr != nil {
				return nil, parseErr
			}
			createdAt, parseErr := parseTime(row.CreatedAt)
			if parseErr != nil {
				return nil, parseErr
			}
			out.Add(DashboardResult{
				ID:              row.ID,
				MonitorID:       row.MonitorID,
				AgentID:         row.AgentID,
				AgentName:       agentNames[row.AgentID],
				RegionCode:      regions[row.RegionID],
				EnvironmentCode: environments[row.EnvironmentID],
				GroupName:       monitorGroups[row.MonitorID],
				Status:          model.Status(row.Status),
				Latency:         time.Duration(row.LatencyMS) * time.Millisecond,
				ErrorMessage:    row.ErrorMessage,
				CheckedAt:       checkedAt,
				CreatedAt:       createdAt,
			})
			return out, nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("build dashboard results: %w", err)
	}
	return results.Values(), nil
}

func dashboardAgentNames(agents []DashboardAgent) map[string]string {
	return collectionlist.ReduceList(collectionlist.NewList(agents...), make(map[string]string, len(agents)), func(out map[string]string, _ int, agent DashboardAgent) map[string]string {
		out[agent.ID] = agent.Name
		return out
	})
}

func dashboardMonitorGroups(monitors []DashboardMonitor) map[string]string {
	return collectionlist.ReduceList(collectionlist.NewList(monitors...), make(map[string]string, len(monitors)), func(out map[string]string, _ int, monitor DashboardMonitor) map[string]string {
		out[monitor.ID] = monitor.GroupName
		return out
	})
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
