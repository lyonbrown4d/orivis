package api

import (
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func dashboardLatestResults(results []store.DashboardResult) map[string]dashboardResultView {
	latestByMonitor := make(map[string]dashboardResultView, len(results))
	collectionlist.NewList(results...).Range(func(_ int, result store.DashboardResult) bool {
		if _, ok := latestByMonitor[result.MonitorID]; !ok {
			latestByMonitor[result.MonitorID] = dashboardResultView{DashboardResult: result}
		}
		return true
	})
	return latestByMonitor
}

func dashboardMonitorViews(
	monitors []store.DashboardMonitor,
	latestByMonitor map[string]dashboardResultView,
) []dashboardMonitorView {
	return collectionlist.MapList(
		collectionlist.NewList(monitors...),
		func(_ int, monitor store.DashboardMonitor) dashboardMonitorView {
			item := dashboardMonitorView{DashboardMonitor: monitor}
			if latest, ok := latestByMonitor[monitor.ID]; ok {
				latest.MonitorName = monitor.Name
				item.Latest = &latest
			}
			return item
		},
	).Values()
}

func dashboardEnvironmentGroups(monitors []dashboardMonitorView) []dashboardEnvironmentGroup {
	indexByName := make(map[string]int, len(monitors))
	groups := make([]dashboardEnvironmentGroup, 0)
	for index := range monitors {
		monitor := monitors[index]
		group := dashboardEnvironmentGroupForMonitor(&monitor, indexByName, &groups)
		group.Monitors = append(group.Monitors, monitor)
		addDashboardStatus(&group.Up, &group.Down, &group.Unknown, monitor.Latest)
	}
	return groups
}

func dashboardEnvironmentGroupForMonitor(
	monitor *dashboardMonitorView,
	indexByName map[string]int,
	groups *[]dashboardEnvironmentGroup,
) *dashboardEnvironmentGroup {
	name := strings.TrimSpace(monitor.EnvironmentCode)
	if name == "" {
		name = "default"
	}
	index, ok := indexByName[name]
	if !ok {
		index = len(*groups)
		indexByName[name] = index
		*groups = append(*groups, dashboardEnvironmentGroup{Name: name})
	}
	return &(*groups)[index]
}

func dashboardResults(snapshot store.DashboardSnapshot, limit int) []dashboardResultView {
	monitorNames := collectionmapping.AssociateList(
		collectionlist.NewList(snapshot.Monitors...),
		func(_ int, monitor store.DashboardMonitor) (string, string) {
			return monitor.ID, monitor.Name
		},
	)

	results := collectionlist.MapList(
		collectionlist.NewList(snapshot.Results...),
		func(_ int, result store.DashboardResult) dashboardResultView {
			return dashboardResultView{
				DashboardResult: result,
				MonitorName:     monitorNames.GetOrDefault(result.MonitorID, ""),
			}
		},
	).Values()
	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}

func dashboardMonitorStatusTotals(monitors []dashboardMonitorView) (int, int, int) {
	up := 0
	down := 0
	unknown := 0
	for index := range monitors {
		addDashboardStatus(&up, &down, &unknown, monitors[index].Latest)
	}
	return up, down, unknown
}

func addDashboardStatus(up, down, unknown *int, latest *dashboardResultView) {
	switch {
	case latest == nil:
		(*unknown)++
	case latest.Status == model.StatusUp:
		(*up)++
	case latest.Status == model.StatusDown:
		(*down)++
	default:
		(*unknown)++
	}
}
