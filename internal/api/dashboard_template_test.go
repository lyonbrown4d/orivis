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

func TestDashboardTemplateRoutesUseBasePath(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.HTTP.BasePath = "/orivis"
	cfg.DB.Driver = "sqlite"
	server := newAPITestServer(cfg, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	for _, path := range []string{"/orivis", "/orivis/dashboard", "/orivis/status", "/orivis/core"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected template route %s to return 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/orivis/dashboard", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		`href="/orivis/ui/static/app.css"`,
		`src="/orivis/ui/static/app.js"`,
		`href="/orivis/status"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected base path template content %q in body", want)
		}
	}
}

func TestDashboardTemplateStaticAssetUsesBasePath(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.HTTP.BasePath = "/orivis"
	cfg.DB.Driver = "sqlite"
	server := newAPITestServer(cfg, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/orivis/ui/static/app.css", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected base path static asset to return 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "@import") {
		t.Fatalf("expected app.css to contain imports, got %q", rec.Body.String())
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

func TestDashboardTemplateAuthUsesBasePath(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.HTTP.BasePath = "/orivis"
	cfg.DB.Driver = "sqlite"
	cfg.Auth.Dashboard.Enabled = true
	cfg.Auth.Dashboard.Username = "admin"
	cfg.Auth.Dashboard.Password = "secret"
	server := newAPITestServer(cfg, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/orivis/dashboard", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected dashboard to redirect, got %d: %s", rec.Code, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/orivis/login" {
		t.Fatalf("expected base path redirect to login, got %q", location)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/orivis/login", strings.NewReader("username=admin&password=secret"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected login to redirect, got %d: %s", rec.Code, rec.Body.String())
	}
	if location := rec.Header().Get("Location"); location != "/orivis/dashboard" {
		t.Fatalf("expected base path redirect to dashboard, got %q", location)
	}
	if cookie := rec.Header().Get("Set-Cookie"); !strings.Contains(cookie, "Path=/orivis") {
		t.Fatalf("expected dashboard cookie path to use base path, got %q", cookie)
	}
}

func TestLoginTemplateDoesNotRenderProtectedNavigation(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.HTTP.BasePath = "/orivis"
	cfg.DB.Driver = "sqlite"
	cfg.Auth.Dashboard.Enabled = true
	cfg.Auth.Dashboard.Username = "admin"
	cfg.Auth.Dashboard.Password = "secret"
	server := newAPITestServer(cfg, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/orivis/login", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected login page, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, forbidden := range []string{
		`orivis-navbar`,
		`orivis-sidebar`,
		`href="/orivis/dashboard"`,
		`href="/orivis/status"`,
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("login page should not render protected navigation %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `action="/orivis/login"`) {
		t.Fatalf("expected base path login form action: %s", body)
	}
}
