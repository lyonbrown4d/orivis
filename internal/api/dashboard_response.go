package api

import (
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type dashboardSnapshotResponse struct {
	Name          string                     `json:"name"`
	Env           string                     `json:"env"`
	Version       buildinfo.Info             `json:"version"`
	Database      dashboardDatabase          `json:"database"`
	GeneratedAt   time.Time                  `json:"generated_at"`
	AuthEnabled   bool                       `json:"auth_enabled"`
	AllMonitors   int                        `json:"all_monitors"`
	GroupSlug     string                     `json:"group_slug"`
	SelectedGroup string                     `json:"selected_group"`
	Summary       dashboardSummary           `json:"summary"`
	Groups        []dashboardServiceGroup    `json:"groups"`
	Agents        []dashboardAgentResponse   `json:"agents"`
	Monitors      []dashboardMonitorResponse `json:"monitors"`
	RecentResults []dashboardResultResponse  `json:"recent_results"`
	StatusLights  []dashboardLightResponse   `json:"status_lights"`
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
		Groups:        view.Groups,
		Agents:        dashboardAgentResponses(view.Agents),
		Monitors:      dashboardMonitorResponses(view.Monitors),
		RecentResults: dashboardResultResponses(view.RecentResults),
		StatusLights:  dashboardLightResponses(view.StatusLights),
	}
}

func dashboardAgentResponses(agents []store.DashboardAgent) []dashboardAgentResponse {
	return collectionlist.MapList(
		collectionlist.NewList(agents...),
		func(_ int, agent store.DashboardAgent) dashboardAgentResponse {
			return dashboardAgentResponse{
				ID:               agent.ID,
				Name:             agent.Name,
				RegionCode:       agent.RegionCode,
				EnvironmentCodes: agent.EnvironmentCodes,
				RuntimeType:      agent.RuntimeType,
				Version:          agent.Version,
				LastSeenAt:       agent.LastSeenAt,
				Status:           string(agent.Status),
			}
		},
	).Values()
}

func dashboardMonitorResponses(monitors []dashboardMonitorView) []dashboardMonitorResponse {
	return collectionlist.MapList(
		collectionlist.NewList(monitors...),
		func(_ int, monitor dashboardMonitorView) dashboardMonitorResponse {
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
				DiscoverySource:   monitor.DiscoverySource,
				DiscoveryDetail:   monitor.DiscoveryDetail,
			}
			if monitor.Latest != nil {
				latest := dashboardResultResponseFromView(*monitor.Latest)
				item.Latest = &latest
			}
			return item
		},
	).Values()
}

func dashboardResultResponses(results []dashboardResultView) []dashboardResultResponse {
	return collectionlist.MapList(
		collectionlist.NewList(results...),
		func(_ int, result dashboardResultView) dashboardResultResponse {
			return dashboardResultResponseFromView(result)
		},
	).Values()
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

func dashboardLightResponses(lights []dashboardStatusLight) []dashboardLightResponse {
	return collectionlist.MapList(
		collectionlist.NewList(lights...),
		func(_ int, light dashboardStatusLight) dashboardLightResponse {
			return dashboardLightResponse{
				MonitorName: light.MonitorName,
				Status:      string(light.Status),
				LatencyMS:   light.Latency.Milliseconds(),
				CheckedAt:   light.CheckedAt,
			}
		},
	).Values()
}
