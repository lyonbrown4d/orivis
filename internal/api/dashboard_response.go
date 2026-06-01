package api

import (
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type dashboardSnapshotResponse struct {
	Name          string                          `json:"name"`
	Env           string                          `json:"env"`
	Version       buildinfo.Info                  `json:"version"`
	Database      dashboardDatabase               `json:"database"`
	GeneratedAt   time.Time                       `json:"generated_at"`
	AuthEnabled   bool                            `json:"auth_enabled"`
	AllMonitors   int                             `json:"all_monitors"`
	GroupSlug     string                          `json:"group_slug"`
	SelectedGroup string                          `json:"selected_group"`
	Summary       dashboardSummary                `json:"summary"`
	Groups        []dashboardServiceGroup         `json:"groups"`
	Agents        []dashboardAgentResponse        `json:"agents"`
	Monitors      []dashboardMonitorResponse      `json:"monitors"`
	RecentResults []dashboardResultResponse       `json:"recent_results"`
	StatusLights  []dashboardLightResponse        `json:"status_lights"`
	Notifications []dashboardNotificationResponse `json:"notifications"`
}

type dashboardAgentResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	RegionCode       string    `json:"region_code"`
	EnvironmentCodes []string  `json:"environment_codes"`
	RuntimeType      string    `json:"runtime_type"`
	Version          string    `json:"version"`
	LastSeenAt       time.Time `json:"last_seen_at"`
	Status           string    `json:"status"`
}

type dashboardMonitorResponse struct {
	ID                string                   `json:"id"`
	Name              string                   `json:"name"`
	Type              string                   `json:"type"`
	Target            string                   `json:"target"`
	GroupName         string                   `json:"group_name"`
	EnvironmentCode   string                   `json:"environment_code"`
	Enabled           bool                     `json:"enabled"`
	IntervalMS        int64                    `json:"interval_ms"`
	TimeoutMS         int64                    `json:"timeout_ms"`
	RetryCount        int                      `json:"retry_count"`
	AggregationPolicy string                   `json:"aggregation_policy"`
	Source            string                   `json:"source"`
	DiscoverySource   string                   `json:"discovery_source"`
	DiscoveryDetail   string                   `json:"discovery_detail"`
	Latest            *dashboardResultResponse `json:"latest,omitempty"`
}

type dashboardResultResponse struct {
	ID              string    `json:"id"`
	MonitorID       string    `json:"monitor_id"`
	MonitorName     string    `json:"monitor_name"`
	AgentID         string    `json:"agent_id"`
	AgentName       string    `json:"agent_name"`
	RegionCode      string    `json:"region_code"`
	EnvironmentCode string    `json:"environment_code"`
	GroupName       string    `json:"group_name"`
	Status          string    `json:"status"`
	LatencyMS       int64     `json:"latency_ms"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	CheckedAt       time.Time `json:"checked_at"`
	CreatedAt       time.Time `json:"created_at"`
}

type dashboardLightResponse struct {
	MonitorName string    `json:"monitor_name"`
	Status      string    `json:"status"`
	LatencyMS   int64     `json:"latency_ms"`
	CheckedAt   time.Time `json:"checked_at"`
}

type dashboardNotificationResponse struct {
	ID            string    `json:"id"`
	Channel       string    `json:"channel"`
	Event         string    `json:"event"`
	MonitorID     string    `json:"monitor_id"`
	MonitorName   string    `json:"monitor_name"`
	AgentID       string    `json:"agent_id"`
	RegionID      string    `json:"region_id"`
	EnvironmentID string    `json:"environment_id"`
	Status        string    `json:"status"`
	Attempt       int       `json:"attempt"`
	MaxAttempts   int       `json:"max_attempts"`
	HTTPStatus    int       `json:"http_status"`
	DurationMS    int64     `json:"duration_ms"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	CheckedAt     time.Time `json:"checked_at"`
	SentAt        time.Time `json:"sent_at"`
	CreatedAt     time.Time `json:"created_at"`
}

func newDashboardSnapshotResponse(view *dashboardView) dashboardSnapshotResponse {
	return dashboardSnapshotResponse{
		Name:          view.Name,
		Env:           view.Env,
		Version:       view.Version,
		Database:      view.Database,
		GeneratedAt:   view.GeneratedAt,
		AuthEnabled:   view.AuthEnabled,
		AllMonitors:   view.AllMonitors,
		GroupSlug:     view.GroupSlug,
		SelectedGroup: view.SelectedGroup,
		Summary:       view.Summary,
		Groups:        dashboardListValues(view.Groups),
		Agents:        dashboardAgentResponses(view.Agents),
		Monitors:      dashboardMonitorResponses(view.Monitors),
		RecentResults: dashboardResultResponses(view.RecentResults),
		StatusLights:  dashboardLightResponses(view.StatusLights),
		Notifications: dashboardNotificationResponses(view.Notifications),
	}
}

func dashboardListValues[T any](items *collectionlist.List[T]) []T {
	if items == nil {
		return []T{}
	}
	return dashboardSliceValues(items.Values())
}

func dashboardSliceValues[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func dashboardListOrEmpty[T any](items *collectionlist.List[T]) *collectionlist.List[T] {
	if items == nil {
		return collectionlist.NewList[T]()
	}
	return items
}

func dashboardAgentResponses(agents *collectionlist.List[store.DashboardAgent]) []dashboardAgentResponse {
	agents = dashboardListOrEmpty(agents)
	return dashboardSliceValues(collectionlist.MapList(
		agents,
		func(_ int, agent store.DashboardAgent) dashboardAgentResponse {
			return dashboardAgentResponse{
				ID:               agent.ID,
				Name:             agent.Name,
				RegionCode:       agent.RegionCode,
				EnvironmentCodes: dashboardSliceValues(agent.EnvironmentCodes),
				RuntimeType:      agent.RuntimeType,
				Version:          agent.Version,
				LastSeenAt:       agent.LastSeenAt,
				Status:           string(agent.Status),
			}
		},
	).Values())
}

func dashboardMonitorResponses(monitors *collectionlist.List[dashboardMonitorView]) []dashboardMonitorResponse {
	monitors = dashboardListOrEmpty(monitors)
	return dashboardSliceValues(collectionlist.MapList(
		monitors,
		func(_ int, monitor dashboardMonitorView) dashboardMonitorResponse {
			var latest *dashboardResultResponse
			if monitor.Latest != nil {
				result := dashboardResultResponseFromView(*monitor.Latest)
				latest = &result
			}
			return dashboardMonitorResponseFromStore(monitor.DashboardMonitor, latest)
		},
	).Values())
}

func dashboardResultResponses(results *collectionlist.List[dashboardResultView]) []dashboardResultResponse {
	results = dashboardListOrEmpty(results)
	return dashboardSliceValues(collectionlist.MapList(
		results,
		func(_ int, result dashboardResultView) dashboardResultResponse {
			return dashboardResultResponseFromView(result)
		},
	).Values())
}

func dashboardResultResponseFromView(result dashboardResultView) dashboardResultResponse {
	return dashboardResultResponse{
		ID:              result.ID,
		MonitorID:       result.MonitorID,
		MonitorName:     result.MonitorName,
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

func dashboardLightResponses(lights *collectionlist.List[dashboardStatusLight]) []dashboardLightResponse {
	lights = dashboardListOrEmpty(lights)
	return dashboardSliceValues(collectionlist.MapList(
		lights,
		func(_ int, light dashboardStatusLight) dashboardLightResponse {
			return dashboardLightResponse{
				MonitorName: light.MonitorName,
				Status:      string(light.Status),
				LatencyMS:   light.Latency.Milliseconds(),
				CheckedAt:   light.CheckedAt,
			}
		},
	).Values())
}

func dashboardNotificationResponses(notifications *collectionlist.List[dashboardNotificationView]) []dashboardNotificationResponse {
	notifications = dashboardListOrEmpty(notifications)
	return dashboardSliceValues(collectionlist.MapList(
		notifications,
		func(_ int, notification dashboardNotificationView) dashboardNotificationResponse {
			return dashboardNotificationResponse{
				ID:            notification.ID,
				Channel:       notification.Channel,
				Event:         notification.Event,
				MonitorID:     notification.MonitorID,
				MonitorName:   notification.MonitorName,
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
