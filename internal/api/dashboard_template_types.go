package api

import (
	"html/template"
	"io/fs"
)

const (
	dashboardRoute              = "/dashboard"
	statusRoute                 = "/status"
	loginRoute                  = "/login"
	logoutRoute                 = "/logout"
	monitorDetailSlug           = "monitor"
	monitorDetailRoute          = "/" + monitorDetailSlug
	dashboardMonitorDetailRoute = "/dashboard/monitor"
)

type dashboardTemplateRenderer struct {
	dashboard *template.Template
	status    *template.Template
	login     *template.Template
	monitor   *template.Template
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
	Monitor       *dashboardTemplateMonitorDetail
	Results       []dashboardTemplateMonitorDetailResult
	Notifications []dashboardTemplateMonitorDetailNotification
}

type dashboardTemplateLinks struct {
	Dashboard   string
	Login       string
	Logout      string
	LoginSubmit string
	Status      string
	Refresh     string
	Back        string
	Monitor     string
}

type dashboardTemplateUser struct {
	Name string
}

type dashboardTemplateText struct {
	Overview            string
	Dashboard           string
	Status              string
	All                 string
	Login               string
	Logout              string
	Username            string
	Password            string
	Auth                string
	LoginHeading        string
	LoginIntro          string
	LoginFailed         string
	Search              string
	SearchPlaceholder   string
	ClearFilters        string
	StatusUp            string
	StatusDown          string
	StatusUnknown       string
	StatusOther         string
	NoMatches           string
	SortBy              string
	SortByNameAsc       string
	SortByNameDesc      string
	SortByChecked       string
	SortByCheckedNewest string
	SortByCheckedOldest string
	SortByLatency       string
	SortByLatencySlow   string
	SortByLatencyFast   string
	TotalMonitors       string
	HealthyMonitors     string
	FailingMonitors     string
	UnknownMonitors     string
	Agents              string
	Groups              string
	Monitors            string
	RecentResults       string
	PublicStatus        string
	CurrentStatus       string
	EmptyMonitors       string
	EmptyResults        string
	Target              string
	Environment         string
	Source              string
	Latency             string
	CheckedAt           string
	GeneratedAt         string
	Refresh             string
	MonitorDetails      string
	BackToDashboard     string
	BackToStatus        string
	MonitorName         string
	ProbeHistory        string
	NotificationHistory string
	NoDataYet           string
	Configuration       string
	LastChecked         string
	Discovery           string
	Duration            string
	RetryCount          string
	Interval            string
	Timeout             string
	AggregationPolicy   string
	Enabled             string
	MonitoringTarget    string
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
	ID            string
	Name          string
	Type          string
	Target        string
	Group         string
	Environment   string
	Source        string
	Status        string
	StatusClass   string
	CheckedAt     string
	CheckedAtUnix int64
	Latency       string
	LatencyMs     int64
	Error         string
	Lights        []dashboardTemplateLight
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

type dashboardTemplateMonitorDetail struct {
	ID              string
	Name            string
	Type            string
	Target          string
	Group           string
	Environment     string
	Source          string
	Status          string
	StatusClass     string
	StatusText      string
	CheckedAt       string
	Latency         string
	Error           string
	Enabled         bool
	Interval        string
	Timeout         string
	RetryCount      int
	Aggregation     string
	DiscoverySource string
	DiscoveryDetail string
}

type dashboardTemplateMonitorDetailResult struct {
	AgentName   string
	Status      string
	StatusClass string
	CheckedAt   string
	Latency     string
	Error       string
}

type dashboardTemplateMonitorDetailNotification struct {
	ID          string
	Channel     string
	Event       string
	Status      string
	StatusClass string
	Attempt     string
	MaxAttempts string
	HTTPStatus  int
	Duration    string
	Error       string
	SentAt      string
	CheckedAt   string
}

type dashboardTemplateLight struct {
	Status string
	Class  string
	Title  string
}
