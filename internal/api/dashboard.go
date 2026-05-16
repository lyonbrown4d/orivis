package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/danielgtaylor/huma/v2"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type dashboardInput struct {
	Authorization string `header:"Authorization"`
	Lang          string `query:"lang"`
}

type dashboardOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

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
	LangOptions   []dashboardLanguageOption
	RecentResults []dashboardResultView
	Summary       dashboardSummary
	T             func(string) string
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

type dashboardResultView struct {
	store.DashboardResult
	MonitorName string
}

func (s *Server) verifyDashboardAuth(authorization string) error {
	cfg := s.cfg.Auth.Dashboard
	if !cfg.Enabled {
		return nil
	}

	username := strings.TrimSpace(cfg.Username)
	password := cfg.Password
	if username == "" || password == "" {
		return huma.Error500InternalServerError("dashboard auth is enabled but credentials are not configured")
	}

	const prefix = "Basic "
	if !strings.HasPrefix(authorization, prefix) {
		return dashboardUnauthorized()
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(strings.TrimPrefix(authorization, prefix)))
	if err != nil {
		return dashboardUnauthorized()
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return dashboardUnauthorized()
	}

	usernameOK := subtle.ConstantTimeCompare([]byte(parts[0]), []byte(username)) == 1
	passwordOK := subtle.ConstantTimeCompare([]byte(parts[1]), []byte(password)) == 1
	if !usernameOK || !passwordOK {
		return dashboardUnauthorized()
	}
	return nil
}

func dashboardUnauthorized() error {
	return huma.ErrorWithHeaders(
		huma.Error401Unauthorized("dashboard authentication required"),
		http.Header{"WWW-Authenticate": []string{`Basic realm="orivis"`}},
	)
}

func (s *Server) renderDashboard(ctx context.Context, lang string) ([]byte, error) {
	lang = dashboardLocale(lang)
	view := dashboardView{
		Lang:        lang,
		Name:        "orivis-server",
		Env:         s.cfg.App.Env,
		Version:     buildinfo.Current(),
		GeneratedAt: time.Now().UTC(),
		LangOptions: dashboardLangOptions(lang),
		T:           dashboardT(lang),
		Database: dashboardDatabase{
			Driver: s.cfg.DB.Driver,
		},
	}
	if s.store != nil && s.store.DB != nil && s.store.DB.Dialect() != nil {
		view.Database.Dialect = s.store.DB.Dialect().Name()
	}

	if s.store != nil {
		snapshot, err := s.store.DashboardSnapshot(ctx, 50)
		if err != nil {
			return nil, huma.Error500InternalServerError("load dashboard snapshot", err)
		}
		view.GeneratedAt = snapshot.GeneratedAt
		view.Agents = snapshot.Agents
		view.Summary.Agents = len(snapshot.Agents)
		view.Summary.Monitors = len(snapshot.Monitors)
		view.Monitors = dashboardMonitors(snapshot)
		view.Environments = dashboardEnvironmentGroups(view.Monitors)
		view.RecentResults = dashboardResults(snapshot, 12)
		for _, monitor := range view.Monitors {
			switch {
			case monitor.Latest == nil:
				view.Summary.Unknown++
			case monitor.Latest.Status == model.StatusUp:
				view.Summary.Up++
			case monitor.Latest.Status == model.StatusDown:
				view.Summary.Down++
			default:
				view.Summary.Unknown++
			}
		}
	}

	var buf bytes.Buffer
	if err := dashboardTemplate.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("execute dashboard template: %w", err)
	}
	return buf.Bytes(), nil
}

func dashboardMonitors(snapshot store.DashboardSnapshot) []dashboardMonitorView {
	latestByMonitor := collectionmapping.NewMapWithCapacity[string, dashboardResultView](len(snapshot.Results))
	collectionlist.NewList(snapshot.Results...).Range(func(_ int, result store.DashboardResult) bool {
		if _, ok := latestByMonitor.Get(result.MonitorID); !ok {
			latestByMonitor.Set(result.MonitorID, dashboardResultView{DashboardResult: result})
		}
		return true
	})

	return collectionlist.MapList(
		collectionlist.NewList(snapshot.Monitors...),
		func(_ int, monitor store.DashboardMonitor) dashboardMonitorView {
			item := dashboardMonitorView{DashboardMonitor: monitor}
			if latest, ok := latestByMonitor.Get(monitor.ID); ok {
				latest.MonitorName = monitor.Name
				item.Latest = &latest
			}
			return item
		},
	).Values()
}

func dashboardEnvironmentGroups(monitors []dashboardMonitorView) []dashboardEnvironmentGroup {
	indexByName := collectionmapping.NewMapWithCapacity[string, int](len(monitors))
	groups := make([]dashboardEnvironmentGroup, 0)
	for _, monitor := range monitors {
		name := strings.TrimSpace(monitor.EnvironmentCode)
		if name == "" {
			name = "default"
		}
		index, ok := indexByName.Get(name)
		if !ok {
			index = len(groups)
			indexByName.Set(name, index)
			groups = append(groups, dashboardEnvironmentGroup{Name: name})
		}

		group := &groups[index]
		group.Monitors = append(group.Monitors, monitor)
		switch {
		case monitor.Latest == nil:
			group.Unknown++
		case monitor.Latest.Status == model.StatusUp:
			group.Up++
		case monitor.Latest.Status == model.StatusDown:
			group.Down++
		default:
			group.Unknown++
		}
	}
	return groups
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

func dashboardStatusClass(status model.Status) string {
	switch status {
	case model.StatusUp:
		return "bg-emerald-100 text-emerald-700 ring-emerald-200"
	case model.StatusDown:
		return "bg-rose-100 text-rose-700 ring-rose-200"
	case model.StatusDegraded:
		return "bg-amber-100 text-amber-700 ring-amber-200"
	default:
		return "bg-slate-100 text-slate-600 ring-slate-200"
	}
}

func dashboardDuration(value time.Duration) string {
	if value <= 0 {
		return "-"
	}
	if value%time.Second == 0 {
		return fmt.Sprintf("%ds", int(value.Seconds()))
	}
	return value.String()
}

func dashboardJoin(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}

func dashboardSince(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	duration := time.Since(value)
	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	default:
		return value.UTC().Format("2006-01-02 15:04 UTC")
	}
}

func dashboardLocale(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "zh":
		return "zh"
	case "zh-cn":
		return "zh"
	case "zh-hans":
		return "zh"
	default:
		return "en"
	}
}

func dashboardLangOptions(activeLang string) []dashboardLanguageOption {
	return []dashboardLanguageOption{
		{Code: "en", Label: "English", Active: activeLang == "en"},
		{Code: "zh", Label: "中文", Active: activeLang == "zh"},
	}
}

func dashboardT(lang string) func(string) string {
	locales := map[string]map[string]string{
		"en": {
			"page.title":             "Orivis Uptime",
			"page.subtitle":          "Distributed availability observability",
			"status.description":     "A zero-config server view for agents, discovered monitors, and recent probe results.",
			"nav.dashboard":          "Dashboard",
			"nav.monitors":           "Monitors",
			"nav.agents":             "Agents",
			"nav.results":            "Recent results",
			"nav.refresh":            "Auto refresh: 15s",
			"kpis.agents":            "Agents",
			"kpis.monitors":          "Monitors",
			"kpis.up":                "Up",
			"kpis.down":              "Down",
			"kpis.unknown":           "Unknown",
			"kpis.total":             "total",
			"summary.server":         "Server",
			"summary.server.env":     "Environment",
			"summary.storage":        "Storage",
			"summary.storage.driver": "Driver",
			"summary.updated":        "Updated",
			"summary.updated.at":     "auto refresh 15s",
			"section.monitors":       "Monitors",
			"section.agents":         "Agents",
			"section.recent_results": "Recent results",
			"table.name":             "Name",
			"table.target":           "Target",
			"table.environment":      "Environment",
			"table.interval":         "Interval",
			"table.status":           "Status",
			"table.latency":          "Latency",
			"table.enabled":          "Enabled",
			"table.disabled":         "Disabled",
			"label.no_monitors":      "No monitors yet. Start an agent with static config or Docker labels.",
			"label.no_agents":        "No agents registered yet.",
			"label.no_results":       "No probe results yet.",
			"agent.title":            "agent / status",
			"label.env":              "env",
			"label.last_seen":        "last seen",
			"label.up":               "up",
			"label.down":             "down",
			"lang.current":           "Language",
			"meta.group":             "Environment",
			"status.unknown":         "unknown",
		},
		"zh": {
			"page.title":             "Orivis 存活检测",
			"page.subtitle":          "分布式可用性检测面板",
			"status.description":     "用于 Agent、自动发现监控与最新探测结果的零配置视图。",
			"nav.dashboard":          "概览",
			"nav.monitors":           "监控列表",
			"nav.agents":             "节点",
			"nav.results":            "最近结果",
			"nav.refresh":            "自动刷新：15秒",
			"kpis.agents":            "节点数",
			"kpis.monitors":          "监控数",
			"kpis.up":                "正常",
			"kpis.down":              "异常",
			"kpis.unknown":           "未知",
			"kpis.total":             "总计",
			"summary.server":         "服务",
			"summary.server.env":     "环境",
			"summary.storage":        "存储",
			"summary.storage.driver": "驱动",
			"summary.updated":        "更新时间",
			"summary.updated.at":     "15秒自动刷新",
			"section.monitors":       "监控",
			"section.agents":         "节点",
			"section.recent_results": "最近结果",
			"table.name":             "名称",
			"table.target":           "目标",
			"table.environment":      "环境",
			"table.interval":         "间隔",
			"table.status":           "状态",
			"table.latency":          "耗时",
			"table.enabled":          "启用",
			"table.disabled":         "停用",
			"label.no_monitors":      "暂无监控。可启动 Agent 并使用静态配置或 Docker 标签。",
			"label.no_agents":        "暂无已注册节点。",
			"label.no_results":       "暂无探测结果。",
			"agent.title":            "节点 / 状态",
			"label.env":              "环境",
			"label.last_seen":        "最后上报",
			"label.up":               "正常",
			"label.down":             "异常",
			"meta.group":             "环境分组",
			"status.unknown":         "未知",
		},
	}
	return func(key string) string {
		if text, ok := locales[lang][key]; ok {
			return text
		}
		return locales["en"][key]
	}
}

var (
	//go:embed "templates/*.tmpl"
	dashboardTemplateFS embed.FS

	dashboardTemplate = template.Must(
		template.New("layout.tmpl").
			Funcs(template.FuncMap{
				"statusClass": dashboardStatusClass,
				"duration":    dashboardDuration,
				"join":        dashboardJoin,
				"since":       dashboardSince,
			}).
			ParseFS(
				dashboardTemplateFS,
				"templates/layout.tmpl",
				"templates/dashboard.tmpl",
			),
	)
)
