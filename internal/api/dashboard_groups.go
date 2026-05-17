package api

import (
	"strings"

	collectionlist "github.com/arcgolabs/collectionx/list"
)

func dashboardEnvironmentGroups(monitors *collectionlist.List[dashboardMonitorView]) *collectionlist.List[dashboardEnvironmentGroup] {
	state := collectionlist.ReduceList(
		monitors,
		dashboardEnvironmentGroupState{
			indexByName: map[string]int{},
			groups:      []dashboardEnvironmentGroup{},
		},
		func(state dashboardEnvironmentGroupState, _ int, monitor dashboardMonitorView) dashboardEnvironmentGroupState {
			group := dashboardEnvironmentGroupForMonitor(&monitor, state.indexByName, &state.groups)
			group.Monitors = append(group.Monitors, monitor)
			addDashboardStatus(&group.Up, &group.Down, &group.Unknown, monitor.Latest)
			return state
		},
	)
	return collectionlist.NewList(state.groups...)
}

type dashboardEnvironmentGroupState struct {
	indexByName map[string]int
	groups      []dashboardEnvironmentGroup
}

func dashboardServiceGroups(monitors *collectionlist.List[dashboardMonitorView], activeSlug string) *collectionlist.List[dashboardServiceGroup] {
	state := collectionlist.ReduceList(
		monitors,
		dashboardServiceGroupState{
			activeSlug:  activeSlug,
			indexBySlug: map[string]int{},
			groups:      []dashboardServiceGroup{},
		},
		func(state dashboardServiceGroupState, _ int, monitor dashboardMonitorView) dashboardServiceGroupState {
			group := dashboardServiceGroupForMonitor(monitor, &state)
			group.Count++
			addDashboardStatus(&group.Up, &group.Down, &group.Unknown, monitor.Latest)
			return state
		},
	)
	return collectionlist.NewList(state.groups...)
}

type dashboardServiceGroupState struct {
	activeSlug  string
	indexBySlug map[string]int
	groups      []dashboardServiceGroup
}

func dashboardServiceGroupForMonitor(monitor dashboardMonitorView, state *dashboardServiceGroupState) *dashboardServiceGroup {
	name := dashboardGroupName(monitor.GroupName)
	slug := dashboardGroupSlug(name)
	groupIndex, ok := state.indexBySlug[slug]
	if !ok {
		groupIndex = len(state.groups)
		state.indexBySlug[slug] = groupIndex
		state.groups = append(state.groups, dashboardServiceGroup{
			Name:   name,
			Slug:   slug,
			Active: slug == state.activeSlug,
		})
	}
	return &state.groups[groupIndex]
}

func dashboardSelectedGroupName(groups *collectionlist.List[dashboardServiceGroup], activeSlug string) string {
	activeSlug = strings.TrimSpace(activeSlug)
	if activeSlug == "" {
		return ""
	}
	var selected string
	groups.Range(func(_ int, group dashboardServiceGroup) bool {
		if group.Slug == activeSlug {
			selected = group.Name
			return false
		}
		return true
	})
	if selected != "" {
		return selected
	}
	return activeSlug
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
