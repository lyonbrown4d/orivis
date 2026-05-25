package api

import (
	"html/template"
	"io/fs"
)

const (
	dashboardRoute = "/dashboard"
	statusRoute    = "/status"
	loginRoute     = "/login"
	logoutRoute    = "/logout"
)

type dashboardTemplateRenderer struct {
	dashboard *template.Template
	status    *template.Template
	login     *template.Template
	static    fs.FS
}

type dashboardTemplatePage struct {
	Locale        string
	Title         string
	Public        bool
	AuthEnabled   bool
	AutoRefresh   bool
	User          *dashboardTemplateUser
	Links         dashboardTemplateLinks
	Text          dashboardTemplateText
	Summary       dashboardTemplateSummary
	Groups        []dashboardServiceGroup
	Agents        []dashboardTemplateAgent
	Monitors      []dashboardTemplateMonitor
	RecentResults []dashboardTemplateResult
	StatusLights  []dashboardTemplateLight
	SelectedGroup string
	GeneratedAt   string
	Error         string
}

type dashboardTemplateLinks struct {
	Dashboard   string
	Login       string
	Logout      string
	LoginSubmit string
	Status      string
	Refresh     string
}

type dashboardTemplateUser struct {
	Name string
}

type dashboardTemplateText struct {
	Overview        string
	Dashboard       string
	Status          string
	Login           string
	Logout          string
	Username        string
	Password        string
	Auth            string
	LoginHeading    string
	LoginIntro      string
	LoginFailed     string
	TotalMonitors   string
	HealthyMonitors string
	FailingMonitors string
	UnknownMonitors string
	Agents          string
	Groups          string
	Monitors        string
	RecentResults   string
	PublicStatus    string
	CurrentStatus   string
	EmptyMonitors   string
	EmptyResults    string
	Target          string
	Environment     string
	Source          string
	Latency         string
	CheckedAt       string
	GeneratedAt     string
	Refresh         string
}

type dashboardTemplateSummary struct {
	Agents   int
	Monitors int
	Up       int
	Down     int
	Unknown  int
}

type dashboardTemplateAgent struct {
	Name         string
	Runtime      string
	Region       string
	Environments string
	Status       string
	LastSeen     string
}

type dashboardTemplateMonitor struct {
	Name        string
	Type        string
	Target      string
	Group       string
	Environment string
	Source      string
	Status      string
	StatusClass string
	CheckedAt   string
	Latency     string
	Error       string
	Lights      []dashboardTemplateLight
}

type dashboardTemplateResult struct {
	MonitorName string
	AgentName   string
	Status      string
	StatusClass string
	CheckedAt   string
	Latency     string
	Error       string
}

type dashboardTemplateLight struct {
	Status string
	Class  string
	Title  string
}
