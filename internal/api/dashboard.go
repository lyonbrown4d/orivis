package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/lyonbrown4d/orivis/internal/buildinfo"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func (e *dashboardEndpoint) verifyDashboardAuth(authorization string) error {
	cfg := e.cfg.Auth.Dashboard
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
	err := huma.ErrorWithHeaders(
		huma.Error401Unauthorized("dashboard authentication required"),
		http.Header{"WWW-Authenticate": []string{`Basic realm="orivis"`}},
	)
	return fmt.Errorf("dashboard unauthorized: %w", err)
}

func (e *dashboardEndpoint) renderDashboard(ctx context.Context, lang string) (*dashboardView, error) {
	lang = dashboardLocale(lang)
	view := dashboardView{
		Lang:        lang,
		Name:        "orivis-server",
		Env:         e.cfg.App.Env,
		Version:     buildinfo.Current(),
		GeneratedAt: time.Now().UTC(),
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
	if err := e.applyDashboardSnapshot(ctx, &view); err != nil {
		return nil, err
	}
	return &view, nil
}

func (e *dashboardEndpoint) applyDashboardSnapshot(ctx context.Context, view *dashboardView) error {
	snapshot, err := e.store.DashboardSnapshot(ctx, 50)
	if err != nil {
		return huma.Error500InternalServerError("load dashboard snapshot", err)
	}
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

func (e *dashboardEndpoint) renderDashboardPage(ctx context.Context, lang string) ([]byte, error) {
	view, err := e.renderDashboard(ctx, lang)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := dashboardTemplate().Render(&buf, "dashboard", view, "layout"); err != nil {
		return nil, fmt.Errorf("render dashboard page: %w", err)
	}
	return buf.Bytes(), nil
}

type dashboardInput struct {
	Authorization string `header:"Authorization"`
	Lang          string `query:"lang"`
}

type dashboardOutput struct {
	ContentType string `header:"Content-Type"`
	Body        []byte
}

func dashboardMonitors(snapshot store.DashboardSnapshot) []dashboardMonitorView {
	latestByMonitor := dashboardLatestResults(snapshot.Results)
	return dashboardMonitorViews(snapshot.Monitors, latestByMonitor)
}
