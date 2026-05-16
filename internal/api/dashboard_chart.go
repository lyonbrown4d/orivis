package api

import (
	"slices"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func dashboardStatusLights(snapshot store.DashboardSnapshot, limit int) []dashboardStatusLight {
	monitorNames := dashboardMonitorNameMap(snapshot.Monitors)
	results := snapshot.Results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	ordered := make([]store.DashboardResult, 0, len(results))
	for index := range slices.Backward(results) {
		ordered = append(ordered, results[index])
	}
	return collectionlist.MapList(
		collectionlist.NewList(ordered...),
		func(_ int, result store.DashboardResult) dashboardStatusLight {
			return dashboardStatusLight{
				MonitorName: monitorNames[result.MonitorID],
				Status:      result.Status,
				Latency:     result.Latency,
				CheckedAt:   result.CheckedAt,
			}
		},
	).Values()
}

func dashboardMonitorNameMap(monitors []store.DashboardMonitor) map[string]string {
	out := make(map[string]string, len(monitors))
	for index := range monitors {
		monitor := monitors[index]
		out[monitor.ID] = monitor.Name
	}
	return out
}
