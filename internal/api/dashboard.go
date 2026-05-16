package api

import (
	"context"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func (e *dashboardEndpoint) dashboardView(ctx context.Context, group string) (*dashboardView, error) {
	groupSlug := ""
	if strings.TrimSpace(group) != "" {
		groupSlug = dashboardGroupSlug(group)
	}
	view := dashboardView{
		Name:        "orivis-server",
		Env:         e.cfg.App.Env,
		Version:     buildinfo.Current(),
		GeneratedAt: time.Now().UTC(),
		AuthEnabled: e.cfg.Auth.Dashboard.Enabled,
		GroupSlug:   groupSlug,
		Database: dashboardDatabase{
			Driver: e.cfg.DB.Driver,
		},
	}
	if e.store != nil && e.store.DB != nil && e.store.DB.Dialect() != nil {
		view.Database.Dialect = e.store.DB.Dialect().Name()
	}
	if e.store == nil {
		return &view, nil
	}
	if err := e.applyDashboardSnapshot(ctx, &view, groupSlug); err != nil {
		return nil, err
	}
	return &view, nil
}

func (e *dashboardEndpoint) applyDashboardSnapshot(ctx context.Context, view *dashboardView, groupSlug string) error {
	snapshot, err := e.store.DashboardSnapshot(ctx, 80)
	if err != nil {
		return huma.Error500InternalServerError("load dashboard snapshot", err)
	}
	allMonitors := dashboardMonitors(snapshot)
	view.AllMonitors = len(allMonitors)
	view.Groups = dashboardServiceGroups(allMonitors, groupSlug)
	view.SelectedGroup = dashboardSelectedGroupName(view.Groups, groupSlug)
	snapshot = dashboardFilteredSnapshot(snapshot, groupSlug)
	view.GeneratedAt = snapshot.GeneratedAt
	view.Agents = snapshot.Agents
	view.Summary.Agents = len(snapshot.Agents)
	view.Summary.Monitors = len(snapshot.Monitors)
	view.Monitors = dashboardMonitors(snapshot)
	view.Environments = dashboardEnvironmentGroups(view.Monitors)
	view.RecentResults = dashboardResults(snapshot, 20)
	view.StatusLights = dashboardStatusLights(snapshot, 120)
	view.Summary.Up, view.Summary.Down, view.Summary.Unknown = dashboardMonitorStatusTotals(view.Monitors)
	return nil
}

func (e *dashboardEndpoint) dashboardSnapshotResponse(ctx context.Context, group string) (dashboardSnapshotResponse, error) {
	view, err := e.dashboardView(ctx, group)
	if err != nil {
		return dashboardSnapshotResponse{}, err
	}
	return newDashboardSnapshotResponse(view), nil
}

func dashboardMonitors(snapshot store.DashboardSnapshot) []dashboardMonitorView {
	latestByMonitor := dashboardLatestResults(snapshot.Results)
	return dashboardMonitorViews(snapshot.Monitors, latestByMonitor)
}
