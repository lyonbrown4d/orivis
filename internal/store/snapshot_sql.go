package store

import (
	"context"
	"slices"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
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
	notifications, err := s.DashboardNotifications(ctx, 20)
	if err != nil {
		return DashboardSnapshot{}, err
	}
	return DashboardSnapshot{
		GeneratedAt:   time.Now().UTC(),
		Agents:        agents,
		Monitors:      monitors,
		Results:       results,
		Notifications: notifications,
	}, nil
}

func (s *Store) sqlDashboardRegions(ctx context.Context) (*collectionmapping.Map[string, string], error) {
	rows, err := s.repositories.regions.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(regionsSchema).Values()...).
			From(regionsSchema),
	)
	if err != nil {
		return nil, wrapError(err, "list dashboard regions")
	}
	return collectionmapping.AssociateList(rows, func(_ int, row regionRow) (string, string) {
		return row.ID, row.Code
	}), nil
}

func (s *Store) sqlDashboardEnvironments(ctx context.Context) (*collectionmapping.Map[string, string], error) {
	rows, err := s.repositories.environments.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(environmentsSchema).Values()...).
			From(environmentsSchema),
	)
	if err != nil {
		return nil, wrapError(err, "list dashboard environments")
	}
	return collectionmapping.AssociateList(rows, func(_ int, row environmentRow) (string, string) {
		return row.ID, row.Code
	}), nil
}

func (s *Store) sqlDashboardAgentEnvironments(
	ctx context.Context,
	environments *collectionmapping.Map[string, string],
) (*collectionmapping.Map[string, []string], error) {
	rows, err := s.repositories.agentEnvironments.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(agentEnvironmentsSchema).Values()...).
			From(agentEnvironmentsSchema).
			OrderBy(agentEnvironmentsSchema.AgentID.Asc(), agentEnvironmentsSchema.EnvironmentID.Asc()),
	)
	if err != nil {
		return nil, wrapError(err, "list dashboard agent environments")
	}
	out := collectionlist.ReduceList(rows, collectionmapping.NewMap[string, []string](), func(out *collectionmapping.Map[string, []string], _ int, row agentEnvironmentRow) *collectionmapping.Map[string, []string] {
		code := environments.GetOrDefault(row.EnvironmentID, "")
		if code == "" {
			return out
		}
		out.Set(row.AgentID, append(out.GetOrDefault(row.AgentID, nil), code))
		return out
	})
	out.Range(func(_ string, values []string) bool {
		slices.Sort(values)
		return true
	})
	return out, nil
}

func (s *Store) sqlDashboardAgents(
	ctx context.Context,
	regions *collectionmapping.Map[string, string],
	agentEnvironments *collectionmapping.Map[string, []string],
) ([]DashboardAgent, error) {
	rows, err := s.repositories.agents.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(agentsSchema).Values()...).
			From(agentsSchema).
			OrderBy(agentsSchema.Name.Asc()),
	)
	if err != nil {
		return nil, wrapError(err, "list dashboard agents")
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
				RegionCode:       regions.GetOrDefault(row.RegionID, ""),
				EnvironmentCodes: slices.Clone(agentEnvironments.GetOrDefault(row.ID, nil)),
				RuntimeType:      row.RuntimeType,
				Version:          row.Version,
				LastSeenAt:       lastSeenAt,
				Status:           model.AgentStatus(row.Status),
			})
			return out, nil
		},
	)
	if err != nil {
		return nil, wrapError(err, "build dashboard agents")
	}
	return agents.Values(), nil
}

func (s *Store) sqlDashboardMonitors(ctx context.Context, environments *collectionmapping.Map[string, string]) ([]DashboardMonitor, error) {
	rows, err := s.repositories.monitors.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(monitorsSchema).Values()...).
			From(monitorsSchema),
	)
	if err != nil {
		return nil, wrapError(err, "list dashboard monitors")
	}
	monitors := collectionlist.MapList(rows, func(_ int, row monitorRecord) DashboardMonitor {
		return DashboardMonitor{
			ID:                row.ID,
			SourceKey:         row.SourceKey,
			Name:              row.Name,
			Type:              model.MonitorType(row.Type),
			Target:            row.Target,
			GroupName:         row.GroupName,
			EnvironmentCode:   environments.GetOrDefault(row.EnvironmentID, ""),
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
	regions *collectionmapping.Map[string, string],
	environments *collectionmapping.Map[string, string],
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
		return nil, wrapError(err, "list dashboard results")
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
				AgentName:       agentNames.GetOrDefault(row.AgentID, ""),
				RegionCode:      regions.GetOrDefault(row.RegionID, ""),
				EnvironmentCode: environments.GetOrDefault(row.EnvironmentID, ""),
				GroupName:       monitorGroups.GetOrDefault(row.MonitorID, ""),
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
		return nil, wrapError(err, "build dashboard results")
	}
	return results.Values(), nil
}

func dashboardAgentNames(agents []DashboardAgent) *collectionmapping.Map[string, string] {
	return collectionmapping.AssociateList(collectionlist.NewList(agents...), func(_ int, agent DashboardAgent) (string, string) {
		return agent.ID, agent.Name
	})
}

func dashboardMonitorGroups(monitors []DashboardMonitor) *collectionmapping.Map[string, string] {
	return collectionmapping.AssociateList(collectionlist.NewList(monitors...), func(_ int, monitor DashboardMonitor) (string, string) {
		return monitor.ID, monitor.GroupName
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
