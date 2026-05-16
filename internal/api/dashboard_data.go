package api

import (
	"fmt"
	"strings"
	"unicode"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/store"
)

const dashboardDefaultGroup = "default"

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
			item := dashboardMonitorView{
				DashboardMonitor: monitor,
				DiscoverySource:  dashboardDiscoverySource(monitor.SourceKey, string(monitor.Source)),
				DiscoveryDetail:  dashboardDiscoveryDetail(monitor.SourceKey),
			}
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

func dashboardServiceGroups(monitors []dashboardMonitorView, activeSlug string) []dashboardServiceGroup {
	indexBySlug := make(map[string]int, len(monitors))
	groups := make([]dashboardServiceGroup, 0)
	for index := range monitors {
		monitor := monitors[index]
		name := dashboardGroupName(monitor.GroupName)
		slug := dashboardGroupSlug(name)
		groupIndex, ok := indexBySlug[slug]
		if !ok {
			groupIndex = len(groups)
			indexBySlug[slug] = groupIndex
			groups = append(groups, dashboardServiceGroup{
				Name:   name,
				Slug:   slug,
				Active: slug == activeSlug,
			})
		}
		group := &groups[groupIndex]
		group.Count++
		addDashboardStatus(&group.Up, &group.Down, &group.Unknown, monitor.Latest)
	}
	return groups
}

func dashboardSelectedGroupName(groups []dashboardServiceGroup, activeSlug string) string {
	activeSlug = strings.TrimSpace(activeSlug)
	if activeSlug == "" {
		return ""
	}
	for index := range groups {
		if groups[index].Slug == activeSlug {
			return groups[index].Name
		}
	}
	return activeSlug
}

func dashboardFilteredSnapshot(snapshot store.DashboardSnapshot, groupSlug string) store.DashboardSnapshot {
	groupSlug = strings.TrimSpace(groupSlug)
	if groupSlug == "" {
		return snapshot
	}

	out := snapshot
	out.Monitors = make([]store.DashboardMonitor, 0, len(snapshot.Monitors))
	monitorIDs := make(map[string]struct{}, len(snapshot.Monitors))
	for index := range snapshot.Monitors {
		monitor := snapshot.Monitors[index]
		if dashboardGroupSlug(monitor.GroupName) != groupSlug {
			continue
		}
		out.Monitors = append(out.Monitors, monitor)
		monitorIDs[monitor.ID] = struct{}{}
	}

	out.Results = make([]store.DashboardResult, 0, len(snapshot.Results))
	for index := range snapshot.Results {
		result := snapshot.Results[index]
		if _, ok := monitorIDs[result.MonitorID]; ok {
			out.Results = append(out.Results, result)
		}
	}
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
