package api

import (
	"strings"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func newLoginTemplatePage(ctx templateFiberContext, endpoint *dashboardEndpoint, message string) dashboardTemplatePage {
	text := dashboardTemplateTexts(ctx)
	return dashboardTemplatePage{
		Locale:      dashboardTemplateLocale(ctx),
		Title:       text.Login,
		Public:      true,
		AuthEnabled: endpoint.cfg.Auth.Dashboard.Enabled,
		Links:       dashboardTemplateLinksFor(endpoint.cfg, ""),
		Text:        text,
		Error:       message,
	}
}

func newDashboardTemplatePage(
	ctx templateFiberContext,
	endpoint *dashboardEndpoint,
	view *dashboardView,
	public bool,
	message string,
) dashboardTemplatePage {
	text := dashboardTemplateTexts(ctx)
	links := dashboardTemplateLinksFor(endpoint.cfg, view.GroupSlug)
	if public {
		links.Refresh = links.Status
		links.Back = links.Status
		links.Monitor = prefixedPath(endpoint.cfg, monitorDetailRoute)
	} else {
		links.Back = links.Dashboard
		links.Monitor = prefixedPath(endpoint.cfg, dashboardMonitorDetailRoute)
	}
	return dashboardTemplatePage{
		Locale:        dashboardTemplateLocale(ctx),
		Title:         dashboardTemplateTitle(text, view, public),
		Public:        public,
		AuthEnabled:   endpoint.cfg.Auth.Dashboard.Enabled,
		AutoRefresh:   true,
		User:          dashboardTemplateUserFor(endpoint, ctx),
		Links:         links,
		Text:          text,
		Summary:       dashboardTemplateSummaryFromView(view.Summary),
		Groups:        dashboardListValues(view.Groups),
		Agents:        dashboardTemplateAgents(dashboardListValues(view.Agents)),
		Monitors:      dashboardTemplateMonitors(dashboardListValues(view.Monitors), dashboardListValues(view.StatusLights)),
		RecentResults: dashboardTemplateResults(dashboardListValues(view.RecentResults)),
		StatusLights:  dashboardTemplateLights(dashboardListValues(view.StatusLights)),
		SelectedGroup: view.SelectedGroup,
		GeneratedAt:   dashboardTemplateTime(view.GeneratedAt),
		Error:         message,
	}
}

func dashboardTemplateSummaryFromView(summary dashboardSummary) dashboardTemplateSummary {
	return dashboardTemplateSummary(summary)
}

func dashboardTemplateTitle(text dashboardTemplateText, view *dashboardView, public bool) string {
	if public {
		if view.SelectedGroup != "" {
			return view.SelectedGroup + " - " + text.PublicStatus
		}
		return text.PublicStatus
	}
	return text.Dashboard
}

func dashboardTemplateUserFor(endpoint *dashboardEndpoint, ctx templateFiberContext) *dashboardTemplateUser {
	if !endpoint.cfg.Auth.Dashboard.Enabled {
		return nil
	}
	claims, ok := endpoint.dashboardClaims(ctx.Cookies(dashboardAuthCookie))
	if !ok {
		return nil
	}
	return &dashboardTemplateUser{Name: claims.Subject}
}

func dashboardTemplateLinksFor(cfg config.Config, group string) dashboardTemplateLinks {
	status := prefixedPath(cfg, statusRoute)
	if group != "" {
		status = prefixedPath(cfg, "/"+group)
	}
	return dashboardTemplateLinks{
		Dashboard:   prefixedPath(cfg, dashboardRoute),
		Login:       prefixedPath(cfg, loginRoute),
		Logout:      prefixedPath(cfg, logoutRoute),
		LoginSubmit: prefixedPath(cfg, loginRoute),
		Root:        httpBasePath(cfg),
		Static:      prefixedPath(cfg, "/ui/static"),
		Status:      status,
		Refresh:     prefixedPath(cfg, dashboardRoute),
	}
}

func dashboardTemplateAgents(agents []store.DashboardAgent) []dashboardTemplateAgent {
	out := make([]dashboardTemplateAgent, 0, len(agents))
	for index := range agents {
		agent := &agents[index]
		out = append(out, dashboardTemplateAgent{
			Name:         agent.Name,
			Runtime:      agent.RuntimeType,
			Region:       agent.RegionCode,
			Environments: strings.Join(agent.EnvironmentCodes, ", "),
			Status:       string(agent.Status),
			LastSeen:     dashboardTemplateTime(agent.LastSeenAt),
		})
	}
	return out
}

func dashboardTemplateMonitors(monitors []dashboardMonitorView, lights []dashboardStatusLight) []dashboardTemplateMonitor {
	lightsByMonitor := dashboardTemplateLightsByMonitor(lights)
	out := make([]dashboardTemplateMonitor, 0, len(monitors))
	for index := range monitors {
		monitor := &monitors[index]
		latest := dashboardTemplateLatestMetrics(monitor.Latest)
		out = append(out, dashboardTemplateMonitor{
			ID:            monitor.ID,
			Name:          monitor.Name,
			Type:          string(monitor.Type),
			Target:        monitor.Target,
			Group:         dashboardGroupName(monitor.GroupName),
			Environment:   dashboardEnvironmentName(monitor.EnvironmentCode),
			Source:        string(monitor.Source),
			Status:        latest.Status,
			StatusClass:   latest.StatusClass,
			CheckedAt:     latest.CheckedAt,
			CheckedAtUnix: latest.CheckedAtUnix,
			Latency:       latest.Latency,
			LatencyMs:     latest.LatencyMS,
			Error:         latest.ErrorMessage,
			Lights:        lightsByMonitor[monitor.Name],
		})
	}
	return out
}

type dashboardTemplateLatestView struct {
	Status        string
	StatusClass   string
	CheckedAt     string
	Latency       string
	ErrorMessage  string
	LatencyMS     int64
	CheckedAtUnix int64
}

func dashboardTemplateLatestMetrics(result *dashboardResultView) dashboardTemplateLatestView {
	if result == nil {
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

	return dashboardTemplateLatestView{
		Status:        string(result.Status),
		StatusClass:   dashboardTemplateStatusClass(result.Status),
		CheckedAt:     dashboardTemplateTime(result.CheckedAt),
		Latency:       dashboardTemplateDuration(result.Latency),
		ErrorMessage:  result.ErrorMessage,
		LatencyMS:     result.Latency.Milliseconds(),
		CheckedAtUnix: result.CheckedAt.Unix(),
	}
}

func dashboardTemplateResults(results []dashboardResultView) []dashboardTemplateResult {
	out := make([]dashboardTemplateResult, 0, len(results))
	for index := range results {
		result := &results[index]
		out = append(out, dashboardTemplateResult{
			MonitorName: result.MonitorName,
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

func dashboardTemplateLightsByMonitor(lights []dashboardStatusLight) map[string][]dashboardTemplateLight {
	out := make(map[string][]dashboardTemplateLight, len(lights))
	for _, light := range lights {
		out[light.MonitorName] = append(out[light.MonitorName], dashboardTemplateLightFrom(light))
	}
	return out
}

func dashboardTemplateLights(lights []dashboardStatusLight) []dashboardTemplateLight {
	out := make([]dashboardTemplateLight, 0, len(lights))
	for _, light := range lights {
		out = append(out, dashboardTemplateLightFrom(light))
	}
	return out
}

func dashboardTemplateLightFrom(light dashboardStatusLight) dashboardTemplateLight {
	return dashboardTemplateLight{
		Status: string(light.Status),
		Class:  dashboardTemplateStatusClass(light.Status),
		Title:  strings.TrimSpace(light.MonitorName + " " + string(light.Status) + " " + dashboardTemplateTime(light.CheckedAt)),
	}
}

func dashboardTemplateStatusClass(status model.Status) string {
	switch status {
	case model.StatusUp:
		return "success"
	case model.StatusDown:
		return "danger"
	case model.StatusDegraded:
		return "warning"
	case model.StatusUnknown:
		return "secondary"
	}
	return "secondary"
}

func dashboardTemplateDuration(value time.Duration) string {
	if value <= 0 {
		return "-"
	}
	if value < time.Second {
		return value.Round(time.Millisecond).String()
	}
	return value.Round(10 * time.Millisecond).String()
}

func dashboardTemplateTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("2006-01-02 15:04:05")
}
