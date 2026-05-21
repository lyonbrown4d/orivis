package api

import (
	"slices"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
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
				MonitorName: monitorNames.GetOrDefault(result.MonitorID, ""),
				Status:      result.Status,
				Latency:     result.Latency,
				CheckedAt:   result.CheckedAt,
			}
		},
	)
}

func dashboardMonitorNameMap(monitors *collectionlist.List[store.DashboardMonitor]) *collectionmapping.Map[string, string] {
	return collectionmapping.AssociateList(
		monitors,
		func(_ int, monitor store.DashboardMonitor) (string, string) {
			return monitor.ID, monitor.Name
		},
	)
}
