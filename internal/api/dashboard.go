package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
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
	snapshot, err := e.loadDashboardSnapshot(ctx, 80)
	if err != nil {
		return huma.Error500InternalServerError("load dashboard snapshot", err)
	}
	allMonitors := dashboardMonitors(snapshot)
	view.AllMonitors = allMonitors.Len()
	view.Groups = dashboardServiceGroups(allMonitors, groupSlug)
	view.SelectedGroup = dashboardSelectedGroupName(view.Groups, groupSlug)
	snapshot = dashboardFilteredSnapshot(snapshot, groupSlug)
	view.GeneratedAt = snapshot.GeneratedAt
	view.Agents = collectionlist.NewList(snapshot.Agents...)
	view.Summary.Agents = view.Agents.Len()
	view.Monitors = dashboardMonitors(snapshot)
	view.Summary.Monitors = view.Monitors.Len()
	view.Environments = dashboardEnvironmentGroups(view.Monitors)
	view.RecentResults = dashboardResults(snapshot, 20)
	view.StatusLights = dashboardStatusLights(snapshot, 120)
	view.Summary.Up, view.Summary.Down, view.Summary.Unknown = dashboardMonitorStatusTotals(view.Monitors)
	return nil
}

func (e *dashboardEndpoint) loadDashboardSnapshot(ctx context.Context, resultLimit int) (store.DashboardSnapshot, error) {
	if e.cache == nil || e.snapshotTTL <= 0 {
		snapshot, err := e.store.DashboardSnapshot(ctx, resultLimit)
		if err != nil {
			return store.DashboardSnapshot{}, fmt.Errorf("load dashboard snapshot: %w", err)
		}
		return snapshot, nil
	}
	key := fmt.Sprintf("dashboard:snapshot:%d", resultLimit)
	if snapshot, ok := e.cachedDashboardSnapshot(ctx, key); ok {
		return snapshot, nil
	}
	snapshot, err := e.store.DashboardSnapshot(ctx, resultLimit)
	if err != nil {
		return store.DashboardSnapshot{}, fmt.Errorf("load dashboard snapshot: %w", err)
	}
	e.storeDashboardSnapshot(ctx, key, snapshot)
	return snapshot, nil
}

func (e *dashboardEndpoint) cachedDashboardSnapshot(ctx context.Context, key string) (store.DashboardSnapshot, bool) {
	raw, ok, err := e.cache.Get(ctx, key)
	if err != nil || !ok {
		return store.DashboardSnapshot{}, false
	}
	var snapshot store.DashboardSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		if deleteErr := e.cache.Delete(context.WithoutCancel(ctx), key); deleteErr != nil {
			return store.DashboardSnapshot{}, false
		}
		return store.DashboardSnapshot{}, false
	}
	return snapshot, true
}

func (e *dashboardEndpoint) storeDashboardSnapshot(ctx context.Context, key string, snapshot store.DashboardSnapshot) {
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	if err := e.cache.Set(context.WithoutCancel(ctx), key, raw, e.snapshotTTL); err != nil {
		return
	}
}

func (e *dashboardEndpoint) dashboardSnapshotResponse(ctx context.Context, group string) (dashboardSnapshotResponse, error) {
	view, err := e.dashboardView(ctx, group)
	if err != nil {
		return dashboardSnapshotResponse{}, err
	}
	return newDashboardSnapshotResponse(view), nil
}

func dashboardMonitors(snapshot store.DashboardSnapshot) *collectionlist.List[dashboardMonitorView] {
	latestByMonitor := dashboardLatestResults(collectionlist.NewList(snapshot.Results...))
	return dashboardMonitorViews(collectionlist.NewList(snapshot.Monitors...), latestByMonitor)
}
