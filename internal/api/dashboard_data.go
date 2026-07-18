package api

import (
	"fmt"
	"strings"
	"unicode"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	collectionset "github.com/arcgolabs/collectionx/set"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/store"
)

const dashboardDefaultGroup = "default"

func dashboardLatestResults(results *collectionlist.List[store.DashboardResult]) *collectionmapping.Map[string, dashboardResultView] {
	latestByMonitor := collectionmapping.NewMapWithCapacity[string, dashboardResultView](results.Len())
	results.Range(func(_ int, result store.DashboardResult) bool {
		if _, ok := latestByMonitor.Get(result.MonitorID); !ok {
			latestByMonitor.Set(result.MonitorID, dashboardResultView{DashboardResult: result})
		}
		return true
	})
	return latestByMonitor
}

func dashboardMonitorViews(
	monitors *collectionlist.List[store.DashboardMonitor],
	latestByMonitor *collectionmapping.Map[string, dashboardResultView],
) *collectionlist.List[dashboardMonitorView] {
	return collectionlist.MapList(
		monitors,
		func(_ int, monitor store.DashboardMonitor) dashboardMonitorView {
			item := dashboardMonitorView{
				DashboardMonitor: monitor,
				DiscoverySource:  dashboardDiscoverySource(monitor.SourceKey, string(monitor.Source)),
				DiscoveryDetail:  dashboardDiscoveryDetail(monitor.SourceKey),
			}
			if latest, ok := latestByMonitor.Get(monitor.ID); ok {
				latest.MonitorName = monitor.Name
				item.Latest = &latest
			}
			return item
		},
	)
}

func dashboardMonitorNames(snapshot store.DashboardSnapshot) *collectionmapping.Map[string, string] {
	return collectionmapping.AssociateList(
		collectionlist.NewList(snapshot.Monitors...),
		func(_ int, monitor store.DashboardMonitor) (string, string) {
			return monitor.ID, monitor.Name
		},
	)
}

func dashboardFilteredSnapshot(snapshot store.DashboardSnapshot, groupSlug string) store.DashboardSnapshot {
	groupSlug = strings.TrimSpace(groupSlug)
	if groupSlug == "" {
		return snapshot
	}

	out := snapshot
	monitorIDs := collectionset.NewSetWithCapacity[string](len(snapshot.Monitors))
	out.Monitors = collectionlist.FilterMapList(
		collectionlist.NewList(snapshot.Monitors...),
		func(_ int, monitor store.DashboardMonitor) (store.DashboardMonitor, bool) {
			if dashboardGroupSlug(monitor.GroupName) != groupSlug {
				return store.DashboardMonitor{}, false
			}
			monitorIDs.Add(monitor.ID)
			return monitor, true
		},
	).Values()
	out.Results = collectionlist.FilterMapList(
		collectionlist.NewList(snapshot.Results...),
		func(_ int, result store.DashboardResult) (store.DashboardResult, bool) {
			return result, monitorIDs.Contains(result.MonitorID)
		},
	).Values()
	out.Notifications = collectionlist.FilterMapList(
		collectionlist.NewList(snapshot.Notifications...),
		func(_ int, notification store.DashboardNotification) (store.DashboardNotification, bool) {
			return notification, monitorIDs.Contains(notification.MonitorID)
		},
	).Values()
	return out
}

func dashboardGroupName(value string) string {
	name := strings.TrimSpace(value)
	if name == "" {
		return dashboardDefaultGroup
	}
	return name
}

func dashboardGroupSlug(value string) string {
	value = strings.ToLower(dashboardGroupName(value))
	var builder strings.Builder
	lastDash := false
	for _, item := range value {
		nextLastDash, err := writeDashboardGroupSlugRune(&builder, item, lastDash)
		if err != nil {
			return dashboardDefaultGroup
		}
		lastDash = nextLastDash
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return dashboardDefaultGroup
	}
	return slug
}

func writeDashboardGroupSlugRune(builder *strings.Builder, item rune, lastDash bool) (bool, error) {
	switch {
	case unicode.IsLetter(item) || unicode.IsDigit(item):
		_, err := builder.WriteRune(item)
		if err != nil {
			return false, fmt.Errorf("write group slug rune: %w", err)
		}
		return false, nil
	case item == '-' || item == '_':
		_, err := builder.WriteRune(item)
		if err != nil {
			return false, fmt.Errorf("write group slug separator: %w", err)
		}
		return false, nil
	case lastDash:
		return true, nil
	default:
		if err := builder.WriteByte('-'); err != nil {
			return true, fmt.Errorf("write group slug dash: %w", err)
		}
		return true, nil
	}
}

func dashboardResultsFromMonitorNames(snapshot store.DashboardSnapshot, limit int, monitorNames *collectionmapping.Map[string, string]) *collectionlist.List[dashboardResultView] {
	return dashboardMappedViews(
		collectionlist.NewList(snapshot.Results...),
		limit,
		func(_ int, result store.DashboardResult) dashboardResultView {
			return dashboardResultView{
				DashboardResult: result,
				MonitorName:     monitorNames.GetOrDefault(result.MonitorID, ""),
			}
		},
	)
}

func dashboardNotificationsFromMonitorNames(snapshot store.DashboardSnapshot, limit int, monitorNames *collectionmapping.Map[string, string]) *collectionlist.List[dashboardNotificationView] {
	return dashboardMappedViews(
		collectionlist.NewList(snapshot.Notifications...),
		limit,
		func(_ int, notification store.DashboardNotification) dashboardNotificationView {
			return dashboardNotificationView{
				DashboardNotification: notification,
				MonitorName:           monitorNames.GetOrDefault(notification.MonitorID, ""),
			}
		},
	)
}

func dashboardMappedViews[Input any, Output any](
	input *collectionlist.List[Input],
	limit int,
	mapper func(int, Input) Output,
) *collectionlist.List[Output] {
	mapped := collectionlist.MapList(input, mapper)
	if limit <= 0 || mapped.Len() <= limit {
		return mapped
	}
	return collectionlist.FilterMapList(mapped, func(index int, item Output) (Output, bool) {
		return item, index < limit
	})
}

func dashboardDiscoverySource(sourceKey, fallback string) string {
	sourceKey = strings.ToLower(strings.TrimSpace(sourceKey))
	switch {
	case strings.HasPrefix(sourceKey, "docker:swarm:"):
		return "Docker Swarm"
	case strings.HasPrefix(sourceKey, "docker:compose:"):
		return "Docker Compose"
	case strings.HasPrefix(sourceKey, "docker:container:"):
		return "Docker"
	case strings.HasPrefix(sourceKey, "static:"):
		return "Config file"
	}
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		return "Unknown"
	}
	return fallback
}

func dashboardDiscoveryDetail(sourceKey string) string {
	sourceKey = strings.TrimSpace(sourceKey)
	switch {
	case strings.HasPrefix(sourceKey, "docker:compose:"):
		return strings.TrimPrefix(sourceKey, "docker:compose:")
	case strings.HasPrefix(sourceKey, "docker:container:"):
		return strings.TrimPrefix(sourceKey, "docker:container:")
	case strings.HasPrefix(sourceKey, "docker:swarm:"):
		return strings.TrimPrefix(sourceKey, "docker:swarm:")
	case strings.HasPrefix(sourceKey, "static:"):
		return strings.TrimPrefix(sourceKey, "static:")
	default:
		return sourceKey
	}
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
