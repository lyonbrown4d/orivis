package api

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

type templateFiberContext interface {
	Cookies(key string, defaultValue ...string) string
	Get(key string, defaultValue ...string) string
}

var dashboardTemplateTextEN = dashboardTemplateText{
	Overview:            "Overview",
	Dashboard:           "Dashboard",
	Status:              "Status",
	All:                 "All",
	Login:               "Login",
	Logout:              "Logout",
	Username:            "Username",
	Password:            "Password",
	ShowPassword:        "Show password",
	HidePassword:        "Hide password",
	Auth:                "Authentication",
	LoginHeading:        "Sign in to Orivis",
	LoginIntro:          "Use the configured credentials to access the dashboard.",
	LoginFailed:         "Invalid username or password.",
	Search:              "Search",
	SearchPlaceholder:   "Search monitor name, target, or group",
	ClearFilters:        "Clear filters",
	MonitorDetails:      "Monitor details",
	BackToDashboard:     "Back to dashboard",
	BackToStatus:        "Back to status",
	MonitorName:         "Monitor",
	ProbeHistory:        "Probe results",
	NotificationHistory: "Notification history",
	NoDataYet:           "No data yet.",
	Configuration:       "Configuration",
	LastChecked:         "Last checked",
	Discovery:           "Discovery",
	Duration:            "Duration",
	RetryCount:          "Retry count",
	Interval:            "Interval",
	Timeout:             "Timeout",
	AggregationPolicy:   "Aggregation",
	Enabled:             "Enabled",
	MonitoringTarget:    "Target",
	StatusUp:            "Healthy",
	StatusDown:          "Failing",
	StatusUnknown:       "Unknown",
	StatusOther:         "Other",
	NoMatches:           "No matching monitors found.",
	SortBy:              "Sort",
	SortByNameAsc:       "Name (A → Z)",
	SortByNameDesc:      "Name (Z → A)",
	SortByChecked:       "Latest checked",
	SortByCheckedNewest: "Checked time (newest first)",
	SortByCheckedOldest: "Checked time (oldest first)",
	SortByLatency:       "Latency (slow → fast)",
	SortByLatencySlow:   "Latency (slow first)",
	SortByLatencyFast:   "Latency (fast first)",
	TotalMonitors:       "Total monitors",
	HealthyMonitors:     "Healthy",
	FailingMonitors:     "Failing",
	UnknownMonitors:     "Unknown",
	Agents:              "Agents",
	Groups:              "Groups",
	Monitors:            "Monitors",
	RecentResults:       "Recent results",
	PublicStatus:        "Public status",
	CurrentStatus:       "Current service status",
	EmptyMonitors:       "No monitors yet.",
	EmptyResults:        "No results yet.",
	Target:              "Target",
	Environment:         "Environment",
	Source:              "Source",
	Latency:             "Latency",
	CheckedAt:           "Checked at",
	GeneratedAt:         "Generated at",
	Refresh:             "Refresh",
	LiveRefresh:         "Live refresh",
	PauseRefresh:        "Pause refresh",
	ResumeRefresh:       "Resume refresh",
	RefreshPaused:       "Refresh paused",
}

var dashboardTemplateTextZH = func() dashboardTemplateText {
	text := dashboardTemplateTextEN
	text.Overview = "概览"
	text.Dashboard = "控制台"
	text.Status = "状态页"
	text.All = "全部"
	text.Login = "登录"
	text.Logout = "退出"
	text.Username = "用户名"
	text.Password = "密码"
	text.ShowPassword = "显示密码"
	text.HidePassword = "隐藏密码"
	text.Auth = "认证"
	text.LoginHeading = "登录 Orivis"
	text.LoginIntro = "使用配置的账号密码访问控制台。"
	text.LoginFailed = "用户名或密码错误。"
	text.Search = "搜索"
	text.SearchPlaceholder = "搜索服务名 / 目标 / 分组"
	text.ClearFilters = "清空筛选"
	text.MonitorDetails = "监控详情"
	text.BackToDashboard = "返回仪表盘"
	text.BackToStatus = "返回状态页"
	text.MonitorName = "监控项"
	text.ProbeHistory = "探测记录"
	text.NotificationHistory = "告警记录"
	text.NoDataYet = "暂无数据。"
	text.Configuration = "配置信息"
	text.LastChecked = "最近检查"
	text.Discovery = "发现来源"
	text.Duration = "耗时"
	text.RetryCount = "重试次数"
	text.Interval = "检测间隔"
	text.Timeout = "超时时间"
	text.AggregationPolicy = "聚合策略"
	text.Enabled = "启用"
	text.MonitoringTarget = "检测目标"
	text.StatusUp = "正常"
	text.StatusDown = "异常"
	text.StatusUnknown = "未知"
	text.StatusOther = "其他"
	text.NoMatches = "无匹配结果。"
	text.SortBy = "排序"
	text.SortByNameAsc = "名称（A-Z）"
	text.SortByNameDesc = "名称（Z-A）"
	text.SortByChecked = "最近检查（最近优先）"
	text.SortByCheckedNewest = "检查时间（新-旧）"
	text.SortByCheckedOldest = "检查时间（旧-新）"
	text.SortByLatency = "耗时（慢-快）"
	text.SortByLatencySlow = "耗时（慢->快）"
	text.SortByLatencyFast = "耗时（快->慢）"
	text.TotalMonitors = "监控总数"
	text.HealthyMonitors = "正常"
	text.FailingMonitors = "异常"
	text.UnknownMonitors = "未知"
	text.Agents = "Agent"
	text.Groups = "分组"
	text.Monitors = "监控项"
	text.RecentResults = "最近结果"
	text.PublicStatus = "公开状态"
	text.CurrentStatus = "当前服务状态"
	text.EmptyMonitors = "暂无监控项。"
	text.EmptyResults = "暂无结果。"
	text.Target = "目标"
	text.Environment = "环境"
	text.Source = "来源"
	text.Latency = "耗时"
	text.CheckedAt = "检查时间"
	text.GeneratedAt = "生成时间"
	text.Refresh = "刷新"
	text.LiveRefresh = "实时刷新"
	text.PauseRefresh = "暂停刷新"
	text.ResumeRefresh = "恢复刷新"
	text.RefreshPaused = "刷新已暂停"
	return text
}()

func dashboardTemplateLocale(ctx templateFiberContext) string {
	if strings.Contains(strings.ToLower(ctx.Get(fiber.HeaderAcceptLanguage)), "zh") {
		return "zh-CN"
	}
	return "en"
}

func dashboardTemplateTexts(ctx templateFiberContext) dashboardTemplateText {
	if dashboardTemplateLocale(ctx) == "zh-CN" {
		return dashboardTemplateTextZH
	}
	return dashboardTemplateTextEN
}
