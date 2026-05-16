package api

import (
	"time"

	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type dashboardView struct {
	Lang            string
	Name            string
	Env             string
	Version         buildinfo.Info
	Database        dashboardDatabase
	GeneratedAt     time.Time
	Agents          []store.DashboardAgent
	Monitors        []dashboardMonitorView
	Environments    []dashboardEnvironmentGroup
	Groups          []dashboardServiceGroup
	AuthEnabled     bool
	AllMonitors     int
	GroupSlug       string
	SelectedGroup   string
	LangOptions     []dashboardLanguageOption
	RecentResults   []dashboardResultView
	StatusChartJSON string
	Summary         dashboardSummary
	T               func(string) string
}

type dashboardLanguageOption struct {
	Code   string
	Label  string
	Active bool
}

type dashboardDatabase struct {
	Driver  string
	Dialect string
}

type dashboardSummary struct {
	Agents   int
	Monitors int
	Up       int
	Down     int
	Unknown  int
}

type dashboardMonitorView struct {
	store.DashboardMonitor
	Latest *dashboardResultView
}

type dashboardEnvironmentGroup struct {
	Name     string
	Monitors []dashboardMonitorView
	Up       int
	Down     int
	Unknown  int
}

type dashboardServiceGroup struct {
	Name    string
	Slug    string
	Count   int
	Up      int
	Down    int
	Unknown int
	Active  bool
}

type dashboardResultView struct {
	store.DashboardResult
	MonitorName string
}

type dashboardLoginView struct {
	Lang         string
	RedirectPath string
	LangOptions  []dashboardLanguageOption
	T            func(string) string
}
