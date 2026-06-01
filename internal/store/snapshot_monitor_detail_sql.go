package store

import (
	"context"
	"errors"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/arcgolabs/dbx/querydsl"
	repository "github.com/arcgolabs/dbx/repository"
	"github.com/lyonbrown4d/orivis/internal/model"
)

func (s *Store) sqlDashboardMonitorDetail(
	ctx context.Context,
	monitorID string,
	resultLimit int,
	notificationLimit int,
) (DashboardMonitorDetail, error) {
	monitorID = strings.TrimSpace(monitorID)
	if monitorID == "" {
		return DashboardMonitorDetail{}, wrapError(ErrInvalidInput, "monitor id is required")
	}

	regions, err := s.sqlDashboardRegions(ctx)
	if err != nil {
		return DashboardMonitorDetail{}, err
	}
	environments, err := s.sqlDashboardEnvironments(ctx)
	if err != nil {
		return DashboardMonitorDetail{}, err
	}
	agentEnvironments, err := s.sqlDashboardAgentEnvironments(ctx, environments)
	if err != nil {
		return DashboardMonitorDetail{}, err
	}
	agents, err := s.sqlDashboardAgents(ctx, regions, agentEnvironments)
	if err != nil {
		return DashboardMonitorDetail{}, err
	}
	agentNames := dashboardAgentNames(agents)

	monitor, err := s.sqlDashboardMonitor(ctx, monitorID, environments)
	if err != nil {
		return DashboardMonitorDetail{}, err
	}

	results, err := s.sqlDashboardMonitorResults(
		ctx,
		resultLimit,
		monitorID,
		regions,
		environments,
		agentNames,
		monitor.GroupName,
	)
	if err != nil {
		return DashboardMonitorDetail{}, err
	}

	notifications, err := s.sqlDashboardNotificationsByMonitor(ctx, monitorID, notificationLimit)
	if err != nil {
		return DashboardMonitorDetail{}, err
	}

	return DashboardMonitorDetail{
		Monitor:       monitor,
		Results:       results,
		Notifications: notifications,
	}, nil
}

func (s *Store) sqlDashboardMonitor(
	ctx context.Context,
	monitorID string,
	environments *collectionmapping.Map[string, string],
) (DashboardMonitor, error) {
	record, err := s.repositories.monitors.FirstSpec(ctx, repository.Where(monitorsSchema.ID.Eq(monitorID)))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return DashboardMonitor{}, wrapErrorf(ErrNotFound, "monitor %s", monitorID)
		}
		return DashboardMonitor{}, wrapError(err, "get dashboard monitor")
	}
	return dashboardMonitorFromRow(record, environments), nil
}

func (s *Store) sqlDashboardMonitorResults(
	ctx context.Context,
	limit int,
	monitorID string,
	regions *collectionmapping.Map[string, string],
	environments *collectionmapping.Map[string, string],
	agentNames *collectionmapping.Map[string, string],
	groupName string,
) ([]DashboardResult, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.repositories.probeResults.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(probeResultSchemaResource()).Values()...).
			From(probeResultSchemaResource()).
			Where(probeResultSchemaResource().MonitorID.Eq(monitorID)).
			OrderBy(probeResultSchemaResource().CheckedAt.Desc()).
			Limit(limit),
	)
	if err != nil {
		return nil, wrapError(err, "list dashboard monitor results")
	}
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
				GroupName:       groupName,
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
		return nil, wrapError(err, "build dashboard monitor results")
	}
	return results.Values(), nil
}

func (s *Store) sqlDashboardNotificationsByMonitor(
	ctx context.Context,
	monitorID string,
	limit int,
) ([]DashboardNotification, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.repositories.notificationDeliveries.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(notificationDeliverySchemaResource()).Values()...).
			From(notificationDeliverySchemaResource()).
			Where(notificationDeliverySchemaResource().MonitorID.Eq(monitorID)).
			OrderBy(notificationDeliverySchemaResource().CreatedAt.Desc()).
			Limit(limit),
	)
	if err != nil {
		return nil, wrapError(err, "list dashboard notifications by monitor")
	}
	rowsValues := rows.Values()
	notifications := make([]DashboardNotification, 0, len(rowsValues))
	for index := range rowsValues {
		item, parseErr := dashboardNotificationFromRow(rowsValues[index])
		if parseErr != nil {
			return nil, parseErr
		}
		notifications = append(notifications, item)
	}
	return notifications, nil
}
