package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

func TestSPARoutesServeBuiltWebApp(t *testing.T) {
	webRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("<!doctype html><title>orivis</title>"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.Web.Enabled = true
	cfg.Web.Root = webRoot
	server := newAPITestServer(cfg, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	for _, path := range []string{"/", "/groups/core", "/login"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected SPA route %s to return 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "orivis") {
			t.Fatalf("expected SPA index for %s, got %q", path, rec.Body.String())
		}
	}
}

func TestSPARoutesDoNotHandleBackendPaths(t *testing.T) {
	webRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("<!doctype html><title>orivis</title>"), 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.Web.Enabled = true
	cfg.Web.Root = webRoot
	server := newAPITestServer(cfg, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/unknown", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "orivis") {
		t.Fatalf("expected backend path to bypass SPA fallback")
	}
}
