package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

func TestDashboardTemplateHTMXContract(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.HTTP.BasePath = "/orivis"
	cfg.DB.Driver = "sqlite"
	server := newAPITestServer(cfg, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	for _, test := range []struct {
		path  string
		hxGet string
		label string
	}{
		{path: "/orivis/dashboard", hxGet: "/orivis/dashboard", label: "dashboard"},
		{path: "/orivis/status", hxGet: "/orivis/status", label: "status"},
	} {
		body := dashboardTemplateGET(t, handler, test.path, http.StatusOK)
		assertBodyContains(t, test.label, body,
			`<script src="https://cdn.jsdelivr.net/npm/htmx.org@2.0.10/dist/htmx.min.js"`,
			`integrity="sha384-H5SrcfygHmAuTDZphMHqBJLc3FhssKjG7w/CeCpFReSfwBWDTKpkzPP8c+cLsK+V"`,
			`crossorigin="anonymous"`,
			`hx-get="`+test.hxGet+`"`,
			`hx-select="main"`,
			`hx-target="this"`,
			`hx-swap="outerHTML"`,
			`hx-push-url="false"`,
		)
	}

	body := dashboardTemplateGET(t, handler, "/orivis/dashboard", http.StatusOK)
	assertBodyContains(t, "dashboard selectors", body,
		`data-orivis-monitor-root`,
		`id="orivis-monitor-search"`,
		`id="orivis-monitor-sort"`,
		`data-status-filter=`,
		`id="orivis-refresh-indicator"`,
		`id="orivis-refresh-toggle"`,
		`orivis-error-note`,
	)
}

func TestDashboardTemplateNonPollingPages(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.HTTP.BasePath = "/orivis"
	cfg.DB.Driver = "sqlite"
	cfg.Auth.Dashboard.Enabled = true
	cfg.Auth.Dashboard.Username = "admin"
	cfg.Auth.Dashboard.Password = "secret"
	server := newAPITestServer(cfg, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	login := dashboardTemplateGET(t, handler, "/orivis/login", http.StatusOK)
	if strings.Contains(login, "hx-get=") || strings.Contains(login, "hx-trigger=") {
		t.Fatalf("login page should not enable polling: %s", login)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/orivis/dashboard/monitor/example", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected protected monitor detail to redirect, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "hx-get=") || strings.Contains(rec.Body.String(), "hx-trigger=") {
		t.Fatalf("monitor detail redirect should not render polling markup: %s", rec.Body.String())
	}
}

func dashboardTemplateGET(t *testing.T, handler httpHandler, path string, expectedStatus int) string {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != expectedStatus {
		t.Fatalf("expected template route %s to return %d, got %d: %s", path, expectedStatus, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func assertBodyContains(t *testing.T, page, body string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected %s body to contain %q", page, fragment)
		}
	}
}
