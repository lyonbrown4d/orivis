package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	collectionlist "github.com/arcgolabs/collectionx/list"
	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/template/html/v2"
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
	LangOptions   []dashboardLanguageOption
	RecentResults []dashboardResultView
	Summary       dashboardSummary
	T             func(string) string
}

type dashboardInput struct {
	Authorization string `header:"Authorization"`
	Lang          string `query:"lang"`
}

type dashboardOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
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

func (s *Server) renderDashboard(ctx context.Context, lang string) (*dashboardView, error) {
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

	return &view, nil
}

func (s *Server) renderDashboardPage(ctx context.Context, lang string) ([]byte, error) {
	view, err := s.renderDashboard(ctx, lang)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := dashboardTemplate().Render(&buf, "dashboard", view, "layout"); err != nil {
		return nil, fmt.Errorf("render dashboard page: %w", err)
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
	case model.StatusUnknown:
		return "bg-slate-100 text-slate-600 ring-slate-200"
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

func dashboardSince(t func(string) string, value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	duration := time.Since(value)
	switch {
	case duration < time.Minute:
		return dashboardText(t, "time.just_now", "just now")
	case duration < time.Hour:
		return dashboardFormat(t, "time.minutes_ago", "%dm ago", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return dashboardFormat(t, "time.hours_ago", "%dh ago", int(duration.Hours()))
	default:
		return value.UTC().Format("2006-01-02 15:04 UTC")
	}
}

func dashboardText(t func(string) string, key, fallback string) string {
	if t == nil {
		return fallback
	}
	text := t(key)
	if strings.TrimSpace(text) == "" || text == key {
		return fallback
	}
	return text
}

func dashboardFormat(t func(string) string, key, fallback string, args ...any) string {
	return fmt.Sprintf(dashboardText(t, key, fallback), args...)
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
	t := dashboardT(activeLang)
	return []dashboardLanguageOption{
		{Code: "en", Label: dashboardText(t, "lang.option.en", "English"), Active: activeLang == "en"},
		{Code: "zh", Label: dashboardText(t, "lang.option.zh", "\u4e2d\u6587"), Active: activeLang == "zh"},
	}
}

func dashboardT(lang string) func(string) string {
	locales, err := dashboardTranslations()
	if err != nil {
		return func(key string) string {
			return key
		}
	}

	values := locales[lang]
	fallback := locales["en"]

	return func(key string) string {
		if text, ok := values[key]; ok {
			return text
		}
		if text, ok := fallback[key]; ok {
			return text
		}
		return key
	}
}

var (
	//go:embed "templates/*.tmpl" "locales/*.json"
	dashboardTemplateFS embed.FS

	dashboardTemplateEngine = func() *html.Engine {
		templateFS, err := fs.Sub(dashboardTemplateFS, "templates")
		if err != nil {
			panic(fmt.Errorf("load dashboard templates: %w", err))
		}

		engine := html.NewFileSystem(http.FS(templateFS), ".tmpl")
		engine.AddFunc("statusClass", dashboardStatusClass)
		engine.AddFunc("duration", dashboardDuration)
		engine.AddFunc("join", dashboardJoin)
		engine.AddFunc("since", dashboardSince)
		return engine
	}()

	dashboardLocaleCatalog  = make(map[string]map[string]string)
	dashboardLocaleLoadOnce sync.Once
	dashboardLocaleLoadErr  error
)

func newDashboardViews() *html.Engine {
	return dashboardTemplateEngine
}

func dashboardTemplate() *html.Engine {
	return dashboardTemplateEngine
}

func dashboardTranslations() (map[string]map[string]string, error) {
	dashboardLocaleLoadOnce.Do(func() {
		localeFS, err := fs.Sub(dashboardTemplateFS, "locales")
		if err != nil {
			dashboardLocaleLoadErr = fmt.Errorf("load dashboard locale filesystem: %w", err)
			return
		}
		entries, err := fs.ReadDir(localeFS, ".")
		if err != nil {
			dashboardLocaleLoadErr = fmt.Errorf("read dashboard locale files: %w", err)
			return
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if path.Ext(name) != ".json" {
				continue
			}

			locale := strings.TrimSuffix(name, path.Ext(name))
			content, err := fs.ReadFile(localeFS, name)
			if err != nil {
				dashboardLocaleLoadErr = fmt.Errorf("read locale file %s: %w", name, err)
				return
			}
			values := make(map[string]string)
			if err := json.Unmarshal(content, &values); err != nil {
				dashboardLocaleLoadErr = fmt.Errorf("parse locale file %s: %w", name, err)
				return
			}
			dashboardLocaleCatalog[locale] = values
		}
	})

	return dashboardLocaleCatalog, dashboardLocaleLoadErr
}
