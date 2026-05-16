package api

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func (e *dashboardEndpoint) renderDashboard(ctx context.Context, lang, acceptLanguage, group string) (*dashboardView, error) {
	lang = dashboardLocale(lang, acceptLanguage)
	groupSlug := ""
	if strings.TrimSpace(group) != "" {
		groupSlug = dashboardGroupSlug(group)
	}
	view := dashboardView{
		Lang:        lang,
		Name:        "orivis-server",
		Env:         e.cfg.App.Env,
		Version:     buildinfo.Current(),
		GeneratedAt: time.Now().UTC(),
		AuthEnabled: e.cfg.Auth.Dashboard.Enabled,
		GroupSlug:   groupSlug,
		LangOptions: dashboardLangOptions(lang),
		T:           dashboardT(lang),
		Database: dashboardDatabase{
			Driver: e.cfg.DB.Driver,
		},
	}
	if e.store != nil && e.store.DB != nil && e.store.DB.Dialect() != nil {
		view.Database.Dialect = e.store.DB.Dialect().Name()
	}
	if e.store == nil {
		return &view, nil
	}
	if err := e.applyDashboardSnapshot(ctx, &view, groupSlug); err != nil {
		return nil, err
	}
	return &view, nil
}

func (e *dashboardEndpoint) applyDashboardSnapshot(ctx context.Context, view *dashboardView, groupSlug string) error {
	snapshot, err := e.store.DashboardSnapshot(ctx, 50)
	if err != nil {
		return huma.Error500InternalServerError("load dashboard snapshot", err)
	}
	allMonitors := dashboardMonitors(snapshot)
	view.AllMonitors = len(allMonitors)
	view.Groups = dashboardServiceGroups(allMonitors, groupSlug)
	view.SelectedGroup = dashboardSelectedGroupName(view.Groups, groupSlug)
	snapshot = dashboardFilteredSnapshot(snapshot, groupSlug)
	view.GeneratedAt = snapshot.GeneratedAt
	view.Agents = snapshot.Agents
	view.Summary.Agents = len(snapshot.Agents)
	view.Summary.Monitors = len(snapshot.Monitors)
	view.Monitors = dashboardMonitors(snapshot)
	view.Environments = dashboardEnvironmentGroups(view.Monitors)
	view.RecentResults = dashboardResults(snapshot, 12)
	view.StatusChartJSON = dashboardStatusChartJSON(snapshot, 40)
	view.Summary.Up, view.Summary.Down, view.Summary.Unknown = dashboardMonitorStatusTotals(view.Monitors)
	return nil
}

func (e *dashboardEndpoint) renderDashboardPage(ctx context.Context, lang, acceptLanguage, group string) ([]byte, error) {
	view, err := e.renderDashboard(ctx, lang, acceptLanguage, group)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := dashboardTemplate().Render(&buf, "dashboard", view, "layout"); err != nil {
		return nil, fmt.Errorf("render dashboard page: %w", err)
	}
	return buf.Bytes(), nil
}

func (e *dashboardEndpoint) renderLoginPage(lang, acceptLanguage, group string) ([]byte, error) {
	lang = dashboardLocale(lang, acceptLanguage)
	view := dashboardLoginView{
		Lang:         lang,
		RedirectPath: dashboardLoginRedirectPath(group),
		LangOptions:  dashboardLangOptions(lang),
		T:            dashboardT(lang),
	}

	var buf bytes.Buffer
	if err := dashboardTemplate().Render(&buf, "login", view); err != nil {
		return nil, fmt.Errorf("render dashboard login page: %w", err)
	}
	return buf.Bytes(), nil
}

func dashboardLoginRedirectPath(group string) string {
	group = strings.TrimSpace(group)
	if group == "" {
		return "/"
	}
	return dashboardGroupPath(group)
}

type dashboardInput struct {
	AcceptLanguage string `header:"Accept-Language"`
	SessionCookie  string `cookie:"orivis_dashboard_session"`
	Lang           string `query:"lang"`
}

type dashboardGroupInput struct {
	AcceptLanguage string `header:"Accept-Language"`
	SessionCookie  string `cookie:"orivis_dashboard_session"`
	Lang           string `query:"lang"`
	Group          string `path:"group"`
}

type dashboardLoginInput struct {
	AcceptLanguage string `header:"Accept-Language"`
	Lang           string `query:"lang"`
	Body           struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
}

type dashboardLogoutInput struct {
	SessionCookie string `cookie:"orivis_dashboard_session"`
}

type dashboardOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

type dashboardLoginOutput struct {
	SetCookie string `header:"Set-Cookie"`
	Body      struct {
		OK bool `json:"ok"`
	}
}

type dashboardLogoutOutput struct {
	SetCookie string `header:"Set-Cookie"`
	Location  string `header:"Location"`
	Status    int    `status:"302"`
}

func dashboardMonitors(snapshot store.DashboardSnapshot) []dashboardMonitorView {
	latestByMonitor := dashboardLatestResults(snapshot.Results)
	return dashboardMonitorViews(snapshot.Monitors, latestByMonitor)
}
