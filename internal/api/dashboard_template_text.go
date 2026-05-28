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
	Auth:                "Authentication",
	LoginHeading:        "Sign in to Orivis",
	LoginIntro:          "Use the configured credentials to access the dashboard.",
	LoginFailed:         "Invalid username or password.",
	Search:              "Search",
	SearchPlaceholder:   "Search monitor name, target, or group",
	ClearFilters:        "Clear filters",
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
}

var dashboardTemplateTextZH = dashboardTemplateText{
	Overview:            "概览",
	Dashboard:           "控制台",
	Status:              "状态页",
	All:                 "全部",
	Login:               "登录",
	Logout:              "退出",
	Username:            "用户名",
	Password:            "密码",
	Auth:                "认证",
	LoginHeading:        "登录 Orivis",
	LoginIntro:          "使用配置的账号密码访问控制台。",
	LoginFailed:         "用户名或密码错误。",
	Search:              "搜索",
	SearchPlaceholder:   "搜索服务名 / 目标 / 分组",
	ClearFilters:        "清空筛选",
	StatusUp:            "正常",
	StatusDown:          "异常",
	StatusUnknown:       "未知",
	StatusOther:         "其他",
	NoMatches:           "无匹配结果。",
	SortBy:              "排序",
	SortByNameAsc:       "名称（A-Z）",
	SortByNameDesc:      "名称（Z-A）",
	SortByChecked:       "最近检查（最近优先）",
	SortByCheckedNewest: "检查时间（新-旧）",
	SortByCheckedOldest: "检查时间（旧-新）",
	SortByLatency:       "耗时（慢-快）",
	SortByLatencySlow:   "耗时（慢->快）",
	SortByLatencyFast:   "耗时（快->慢）",
	TotalMonitors:       "监控总数",
	HealthyMonitors:     "正常",
	FailingMonitors:     "异常",
	UnknownMonitors:     "未知",
	Agents:              "Agent",
	Groups:              "分组",
	Monitors:            "监控项",
	RecentResults:       "最近结果",
	PublicStatus:        "公开状态",
	CurrentStatus:       "当前服务状态",
	EmptyMonitors:       "暂无监控项。",
	EmptyResults:        "暂无结果。",
	Target:              "目标",
	Environment:         "环境",
	Source:              "来源",
	Latency:             "耗时",
	CheckedAt:           "检查时间",
	GeneratedAt:         "生成时间",
	Refresh:             "刷新",
}

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
