package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

func TestDashboardTemplateRoutesServeEmbeddedUI(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	server := newAPITestServer(cfg, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	for _, path := range []string{"/", "/dashboard", "/status", "/core"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected template route %s to return 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "Orivis") {
			t.Fatalf("expected template content for %s, got %q", path, rec.Body.String())
		}
	}
}

func TestDashboardTemplateStaticAsset(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	server := newAPITestServer(cfg, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	for _, path := range []string{"/ui/static/app.css", "/ui/static/css/app-core.css", "/ui/static/css/app-dark.css"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected static asset %s to return 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
		if path == "/ui/static/app.css" {
			if !strings.Contains(rec.Body.String(), "@import") {
				t.Fatalf("expected app.css to contain imports, got %q", rec.Body.String())
			}
			continue
		}

		if !strings.Contains(rec.Body.String(), "orivis") {
			t.Fatalf("expected embedded css %s, got %q", path, rec.Body.String())
		}
	}
}

func TestDashboardTemplateRedirectsToLoginWhenAuthEnabled(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.Auth.Dashboard.Enabled = true
	cfg.Auth.Dashboard.Username = "admin"
	cfg.Auth.Dashboard.Password = "secret"
	server := newAPITestServer(cfg, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/dashboard", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected dashboard to redirect, got %d: %s", rec.Code, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/login" {
		t.Fatalf("expected redirect to login, got %q", location)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/login", http.NoBody)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected login to return 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Sign in to Orivis") {
		t.Fatalf("expected login content, got %q", rec.Body.String())
	}
}
