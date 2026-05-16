package api

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type dashboardStatusChartPoint struct {
	Time        string  `json:"time"`
	Label       string  `json:"label"`
	MonitorName string  `json:"monitor_name"`
	Status      string  `json:"status"`
	Score       float64 `json:"score"`
	LatencyMS   int64   `json:"latency_ms"`
}

func dashboardStatusChartJSON(snapshot store.DashboardSnapshot, limit int) string {
	points := dashboardStatusChartPoints(snapshot, limit)
	content, err := json.Marshal(points)
	if err != nil {
		return "[]"
	}
	return string(content)
}

func dashboardStatusChartPoints(snapshot store.DashboardSnapshot, limit int) []dashboardStatusChartPoint {
	monitorNames := dashboardMonitorNameMap(snapshot.Monitors)
	results := snapshot.Results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	points := make([]dashboardStatusChartPoint, 0, len(results))
	for index := range slices.Backward(results) {
		result := results[index]
		points = append(points, dashboardStatusChartPoint{
			Time:        result.CheckedAt.UTC().Format(time.RFC3339),
			Label:       result.CheckedAt.UTC().Format("15:04:05"),
			MonitorName: monitorNames[result.MonitorID],
			Status:      string(result.Status),
			Score:       dashboardStatusScore(result.Status),
			LatencyMS:   result.Latency.Milliseconds(),
		})
	}
	return points
}

func dashboardMonitorNameMap(monitors []store.DashboardMonitor) map[string]string {
	out := make(map[string]string, len(monitors))
	for index := range monitors {
		monitor := monitors[index]
		out[monitor.ID] = monitor.Name
	}
	return out
}

func dashboardStatusScore(status model.Status) float64 {
	switch status {
	case model.StatusUp:
		return 1
	case model.StatusDegraded:
		return 0.55
	case model.StatusUnknown:
		return 0.25
	case model.StatusDown:
		return 0
	default:
		return 0.25
	}
}
