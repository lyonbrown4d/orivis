package api

import (
	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type dashboardMonitorDetailResponse struct {
	Monitor       dashboardMonitorResponse        `json:"monitor"`
	Results       []dashboardResultResponse       `json:"results"`
	Notifications []dashboardNotificationResponse `json:"notifications"`
}

func dashboardMonitorResponseFromStore(monitor store.DashboardMonitor, latest *dashboardResultResponse) dashboardMonitorResponse {
	item := dashboardMonitorResponse{
		ID:                monitor.ID,
		Name:              monitor.Name,
		Type:              string(monitor.Type),
		Target:            monitor.Target,
		GroupName:         dashboardGroupName(monitor.GroupName),
		EnvironmentCode:   monitor.EnvironmentCode,
		Enabled:           monitor.Enabled,
		IntervalMS:        monitor.Interval.Milliseconds(),
		TimeoutMS:         monitor.Timeout.Milliseconds(),
		RetryCount:        monitor.RetryCount,
		AggregationPolicy: string(monitor.AggregationPolicy),
		Source:            string(monitor.Source),
		DiscoverySource:   dashboardDiscoverySource(monitor.SourceKey, string(monitor.Source)),
		DiscoveryDetail:   dashboardDiscoveryDetail(monitor.SourceKey),
	}
	if latest != nil {
		item.Latest = latest
	}
	return item
}

func newDashboardMonitorDetailResponse(detail store.DashboardMonitorDetail) dashboardMonitorDetailResponse {
	var latest *dashboardResultResponse
	if len(detail.Results) > 0 {
		result := dashboardResultResponseFromStore(detail.Results[0], detail.Monitor.Name)
		latest = &result
	}
	return dashboardMonitorDetailResponse{
		Monitor:       dashboardMonitorResponseFromStore(detail.Monitor, latest),
		Results:       dashboardResultResponsesFromStore(detail.Results, detail.Monitor.Name),
		Notifications: dashboardNotificationResponsesFromStore(detail.Notifications, detail.Monitor.Name),
	}
}

func dashboardResultResponsesFromStore(results []store.DashboardResult, monitorName string) []dashboardResultResponse {
	return dashboardSliceValues(collectionlist.MapList(
		collectionlist.NewList(results...),
		func(_ int, result store.DashboardResult) dashboardResultResponse {
			return dashboardResultResponseFromStore(result, monitorName)
		},
	).Values())
}

func dashboardResultResponseFromStore(result store.DashboardResult, monitorName string) dashboardResultResponse {
	return dashboardResultResponse{
		ID:              result.ID,
		MonitorID:       result.MonitorID,
		MonitorName:     monitorName,
		AgentID:         result.AgentID,
		AgentName:       result.AgentName,
		RegionCode:      result.RegionCode,
		EnvironmentCode: result.EnvironmentCode,
		GroupName:       dashboardGroupName(result.GroupName),
		Status:          string(result.Status),
		LatencyMS:       result.Latency.Milliseconds(),
		ErrorMessage:    result.ErrorMessage,
		CheckedAt:       result.CheckedAt,
		CreatedAt:       result.CreatedAt,
	}
}

func dashboardNotificationResponsesFromStore(notifications []store.DashboardNotification, monitorName string) []dashboardNotificationResponse {
	return dashboardSliceValues(collectionlist.MapList(
		collectionlist.NewList(notifications...),
		func(_ int, notification store.DashboardNotification) dashboardNotificationResponse {
			return dashboardNotificationResponse{
				ID:            notification.ID,
				Channel:       notification.Channel,
				Event:         notification.Event,
				MonitorID:     notification.MonitorID,
				MonitorName:   monitorName,
				AgentID:       notification.AgentID,
				RegionID:      notification.RegionID,
				EnvironmentID: notification.EnvironmentID,
				Status:        notification.Status,
				Attempt:       notification.Attempt,
				MaxAttempts:   notification.MaxAttempts,
				HTTPStatus:    notification.HTTPStatus,
				DurationMS:    notification.Duration.Milliseconds(),
				ErrorMessage:  notification.ErrorMessage,
				CheckedAt:     notification.CheckedAt,
				SentAt:        notification.SentAt,
				CreatedAt:     notification.CreatedAt,
			}
		},
	).Values())
}
