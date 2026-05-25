package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

type templateFiberContext interface {
	Cookies(key string, defaultValue ...string) string
	Get(key string, defaultValue ...string) string
}

func dashboardTemplateLocale(ctx templateFiberContext) string {
	if strings.Contains(strings.ToLower(ctx.Get(fiber.HeaderAcceptLanguage)), "zh") {
		return "zh-CN"
	}
	return "en"
}

func dashboardTemplateTexts(ctx templateFiberContext) dashboardTemplateText {
	if dashboardTemplateLocale(ctx) == "zh-CN" {
		return dashboardTemplateText{
			Overview:        "概览",
			Dashboard:       "控制台",
			Status:          "状态页",
			Login:           "登录",
			Logout:          "退出",
			Username:        "用户名",
			Password:        "密码",
			Auth:            "认证",
			LoginHeading:    "登录 Orivis",
			LoginIntro:      "使用配置的账号密码访问控制台。",
			LoginFailed:     "用户名或密码错误。",
			TotalMonitors:   "监控总数",
			HealthyMonitors: "正常",
			FailingMonitors: "异常",
			UnknownMonitors: "未知",
			Agents:          "Agent",
			Groups:          "分组",
			Monitors:        "监控项",
			RecentResults:   "最近结果",
			PublicStatus:    "公开状态",
			CurrentStatus:   "当前服务状态",
			EmptyMonitors:   "暂无监控项。",
			EmptyResults:    "暂无结果。",
			Target:          "目标",
			Environment:     "环境",
			Source:          "来源",
			Latency:         "耗时",
			CheckedAt:       "检查时间",
			GeneratedAt:     "生成时间",
			Refresh:         "刷新",
		}
	}
	return dashboardTemplateText{
		Overview:        "Overview",
		Dashboard:       "Dashboard",
		Status:          "Status",
		Login:           "Login",
		Logout:          "Logout",
		Username:        "Username",
		Password:        "Password",
		Auth:            "Authentication",
		LoginHeading:    "Sign in to Orivis",
		LoginIntro:      "Use the configured credentials to access the dashboard.",
		LoginFailed:     "Invalid username or password.",
		TotalMonitors:   "Total monitors",
		HealthyMonitors: "Healthy",
		FailingMonitors: "Failing",
		UnknownMonitors: "Unknown",
		Agents:          "Agents",
		Groups:          "Groups",
		Monitors:        "Monitors",
		RecentResults:   "Recent results",
		PublicStatus:    "Public status",
		CurrentStatus:   "Current service status",
		EmptyMonitors:   "No monitors yet.",
		EmptyResults:    "No results yet.",
		Target:          "Target",
		Environment:     "Environment",
		Source:          "Source",
		Latency:         "Latency",
		CheckedAt:       "Checked at",
		GeneratedAt:     "Generated at",
		Refresh:         "Refresh",
	}
}
