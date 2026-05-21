package api

import (
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/samber/mo"
)

func dashboardEnvironmentGroups(monitors *collectionlist.List[dashboardMonitorView]) *collectionlist.List[dashboardEnvironmentGroup] {
	groups := collectionmapping.NewOrderedMapWithCapacity[string, dashboardEnvironmentGroup](monitors.Len())
	monitors.Range(func(_ int, monitor dashboardMonitorView) bool {
		name := dashboardEnvironmentName(monitor.EnvironmentCode)
		group := groups.GetOption(name).OrElse(dashboardEnvironmentGroup{Name: name})
		group.Monitors = append(group.Monitors, monitor)
		addDashboardStatus(&group.Up, &group.Down, &group.Unknown, monitor.Latest)
		groups.Set(name, group)
		return true
	})
	return collectionlist.NewList(groups.Values()...)
}

func dashboardServiceGroups(monitors *collectionlist.List[dashboardMonitorView], activeSlug string) *collectionlist.List[dashboardServiceGroup] {
	groups := collectionmapping.NewOrderedMapWithCapacity[string, dashboardServiceGroup](monitors.Len())
	monitors.Range(func(_ int, monitor dashboardMonitorView) bool {
		name := dashboardGroupName(monitor.GroupName)
		slug := dashboardGroupSlug(name)
		group := groups.GetOption(slug).OrElse(dashboardServiceGroup{
			Name:   name,
			Slug:   slug,
			Active: slug == activeSlug,
		})
		group.Count++
		addDashboardStatus(&group.Up, &group.Down, &group.Unknown, monitor.Latest)
		groups.Set(slug, group)
		return true
	})
	return collectionlist.NewList(groups.Values()...)
}

func dashboardSelectedGroupName(groups *collectionlist.List[dashboardServiceGroup], activeSlug string) string {
	activeSlug = strings.TrimSpace(activeSlug)
	if activeSlug == "" {
		return ""
	}
	return mo.TupleToOption(collectionlist.FindList(groups, func(_ int, group dashboardServiceGroup) bool {
		return group.Slug == activeSlug
	})).OrElse(dashboardServiceGroup{Name: activeSlug}).Name
}

func dashboardEnvironmentName(value string) string {
	name := strings.TrimSpace(value)
	if name == "" {
		return "default"
	}
	return name
}

func dashboardMonitorStatusTotals(monitors *collectionlist.List[dashboardMonitorView]) (int, int, int) {
	totals := collectionlist.ReduceList(
		monitors,
		dashboardStatusTotals{},
		func(totals dashboardStatusTotals, _ int, monitor dashboardMonitorView) dashboardStatusTotals {
			addDashboardStatus(&totals.up, &totals.down, &totals.unknown, monitor.Latest)
			return totals
		},
	)
	return totals.up, totals.down, totals.unknown
}

type dashboardStatusTotals struct {
	up      int
	down    int
	unknown int
}
