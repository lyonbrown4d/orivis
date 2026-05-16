package api

import (
	"time"

	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type dashboardView struct {
	Lang          string
	Name          string
	Env           string
	Version       buildinfo.Info
	Database      dashboardDatabase
	GeneratedAt   time.Time
	Agents        []store.DashboardAgent
	Monitors      []dashboardMonitorView
	Environments  []dashboardEnvironmentGroup
	Groups        []dashboardServiceGroup
	AuthEnabled   bool
	AllMonitors   int
	GroupSlug     string
	SelectedGroup string
	RefreshPath   string
	LangOptions   []dashboardLanguageOption
	RecentResults []dashboardResultView
	StatusLights  []dashboardStatusLight
	Summary       dashboardSummary
	T             func(string) string
}

type dashboardLanguageOption struct {
	Code   string
	Label  string
	Active bool
}

type dashboardDatabase struct {
	Driver  string `json:"driver"`
	Dialect string `json:"dialect,omitempty"`
}

type dashboardSummary struct {
	Agents   int `json:"agents"`
	Monitors int `json:"monitors"`
	Up       int `json:"up"`
	Down     int `json:"down"`
	Unknown  int `json:"unknown"`
}

type dashboardMonitorView struct {
	store.DashboardMonitor
	DiscoverySource string
	DiscoveryDetail string
	Latest          *dashboardResultView
}

type dashboardEnvironmentGroup struct {
	Name     string
	Monitors []dashboardMonitorView
	Up       int
	Down     int
	Unknown  int
}

type dashboardServiceGroup struct {
	Name    string `json:"name"`
	Slug    string `json:"slug"`
	Count   int    `json:"count"`
	Up      int    `json:"up"`
	Down    int    `json:"down"`
	Unknown int    `json:"unknown"`
	Active  bool   `json:"active"`
}

type dashboardResultView struct {
	store.DashboardResult
	MonitorName string
}

type dashboardStatusLight struct {
	MonitorName string
	Status      model.Status
	Latency     time.Duration
	CheckedAt   time.Time
}
