package api

import (
	"strconv"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func newDashboardTemplateMonitorDetailPage(
	ctx templateFiberContext,
	endpoint *dashboardEndpoint,
	detail store.DashboardMonitorDetail,
	public bool,
	message string,
) dashboardTemplatePage {
	text := dashboardTemplateTexts(ctx)
	links := dashboardTemplateLinksFor("")
	if public {
		links.Back = links.Status
		links.Monitor = monitorDetailRoute
	} else {
		links.Back = links.Dashboard
		links.Monitor = dashboardMonitorDetailRoute
	}
	return dashboardTemplatePage{
		Locale:        dashboardTemplateLocale(ctx),
		Title:         detail.Monitor.Name,
		Public:        public,
		AuthEnabled:   endpoint.cfg.Auth.Dashboard.Enabled,
		AutoRefresh:   false,
		User:          dashboardTemplateUserFor(endpoint, ctx),
		Links:         links,
		Text:          text,
		Error:         message,
		Monitor:       dashboardTemplateMonitorDetailFromStore(detail),
		Results:       dashboardTemplateMonitorDetailResults(detail.Results),
		Notifications: dashboardTemplateMonitorDetailNotifications(detail.Notifications),
		GeneratedAt:   dashboardTemplateTime(time.Now()),
	}
}

func dashboardTemplateMonitorDetailFromStore(detail store.DashboardMonitorDetail) *dashboardTemplateMonitorDetail {
	latest := dashboardTemplateLatestFromResults(detail.Results)
	return &dashboardTemplateMonitorDetail{
		ID:              detail.Monitor.ID,
		Name:            detail.Monitor.Name,
		Type:            string(detail.Monitor.Type),
		Target:          detail.Monitor.Target,
		Group:           dashboardGroupName(detail.Monitor.GroupName),
		Environment:     dashboardEnvironmentName(detail.Monitor.EnvironmentCode),
		Source:          string(detail.Monitor.Source),
		Status:          latest.Status,
		StatusClass:     latest.StatusClass,
		StatusText:      latest.Status,
		CheckedAt:       latest.CheckedAt,
		Latency:         latest.Latency,
		Error:           latest.ErrorMessage,
		Enabled:         detail.Monitor.Enabled,
		Interval:        detail.Monitor.Interval.String(),
		Timeout:         detail.Monitor.Timeout.String(),
		RetryCount:      detail.Monitor.RetryCount,
		Aggregation:     string(detail.Monitor.AggregationPolicy),
		DiscoverySource: dashboardDiscoverySource(detail.Monitor.SourceKey, string(detail.Monitor.Source)),
		DiscoveryDetail: dashboardDiscoveryDetail(detail.Monitor.SourceKey),
	}
}

func dashboardTemplateLatestFromResults(results []store.DashboardResult) dashboardTemplateLatestView {
	if len(results) == 0 {
		return dashboardTemplateLatestView{
			Status:        string(model.StatusUnknown),
			StatusClass:   "secondary",
			CheckedAt:     "-",
			Latency:       "-",
			ErrorMessage:  "",
			LatencyMS:     0,
			CheckedAtUnix: 0,
		}
	}

	latest := results[0]
	return dashboardTemplateLatestView{
		Status:        string(latest.Status),
		StatusClass:   dashboardTemplateStatusClass(latest.Status),
		CheckedAt:     dashboardTemplateTime(latest.CheckedAt),
		Latency:       dashboardTemplateDuration(latest.Latency),
		ErrorMessage:  latest.ErrorMessage,
		LatencyMS:     latest.Latency.Milliseconds(),
		CheckedAtUnix: latest.CheckedAt.Unix(),
	}
}

func dashboardTemplateMonitorDetailResults(results []store.DashboardResult) []dashboardTemplateMonitorDetailResult {
	out := make([]dashboardTemplateMonitorDetailResult, 0, len(results))
	for index := range results {
		result := &results[index]
		out = append(out, dashboardTemplateMonitorDetailResult{
			AgentName:   result.AgentName,
			Status:      string(result.Status),
			StatusClass: dashboardTemplateStatusClass(result.Status),
			CheckedAt:   dashboardTemplateTime(result.CheckedAt),
			Latency:     dashboardTemplateDuration(result.Latency),
			Error:       result.ErrorMessage,
		})
	}
	return out
}

func dashboardTemplateMonitorDetailNotifications(
	notifications []store.DashboardNotification,
) []dashboardTemplateMonitorDetailNotification {
	out := make([]dashboardTemplateMonitorDetailNotification, 0, len(notifications))
	for index := range notifications {
		item := &notifications[index]
		out = append(out, dashboardTemplateMonitorDetailNotification{
			ID:          item.ID,
			Channel:     item.Channel,
			Event:       item.Event,
			Status:      item.Status,
			StatusClass: dashboardNotificationStatusClass(item.Status),
			Attempt:     strconv.Itoa(item.Attempt),
			MaxAttempts: strconv.Itoa(item.MaxAttempts),
			HTTPStatus:  item.HTTPStatus,
			Duration:    dashboardTemplateDuration(item.Duration),
			Error:       item.ErrorMessage,
			SentAt:      dashboardTemplateTime(item.SentAt),
			CheckedAt:   dashboardTemplateTime(item.CheckedAt),
		})
	}
	return out
}

func dashboardNotificationStatusClass(status string) string {
	switch status {
	case store.NotificationStatusSuccess:
		return "success"
	case store.NotificationStatusFailed:
		return "danger"
	default:
		return "warning"
	}
}
