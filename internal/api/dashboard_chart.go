package api

import (
	"slices"

	collectionlist "github.com/arcgolabs/collectionx/list"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func dashboardStatusLights(snapshot store.DashboardSnapshot, limit int) *collectionlist.List[dashboardStatusLight] {
	monitorNames := dashboardMonitorNameMap(collectionlist.NewList(snapshot.Monitors...))
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
	)
}

func dashboardMonitorNameMap(monitors *collectionlist.List[store.DashboardMonitor]) map[string]string {
	return collectionlist.ReduceList(
		monitors,
		make(map[string]string, monitors.Len()),
		func(out map[string]string, _ int, monitor store.DashboardMonitor) map[string]string {
			out[monitor.ID] = monitor.Name
			return out
		},
	)
}
