package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

func TestRoutesAreRegistered(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	server := newAPITestServer(cfg, nil)

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/server/metadata"},
		{method: http.MethodGet, path: "/healthz"},
		{method: http.MethodGet, path: "/readyz"},
		{method: http.MethodPost, path: "/api/agent/register"},
		{method: http.MethodPost, path: "/api/agent/heartbeat"},
		{method: http.MethodGet, path: "/api/agent/tasks"},
		{method: http.MethodPost, path: "/api/agent/monitors"},
		{method: http.MethodPost, path: "/api/agent/results"},
	}

	for _, tt := range tests {
		if !server.Runtime().HasRoute(tt.method, tt.path) {
			t.Fatalf("expected route %s %s to be registered", tt.method, tt.path)
		}
	}
}

func TestRoutesAreRegisteredWithBasePath(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.HTTP.BasePath = "/orivis"
	cfg.DB.Driver = "sqlite"
	server := newAPITestServer(cfg, nil)

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/orivis/api/server/metadata"},
		{method: http.MethodGet, path: "/orivis/healthz"},
		{method: http.MethodGet, path: "/orivis/readyz"},
		{method: http.MethodPost, path: "/orivis/api/agent/register"},
		{method: http.MethodPost, path: "/orivis/api/agent/heartbeat"},
		{method: http.MethodGet, path: "/orivis/api/agent/tasks"},
		{method: http.MethodPost, path: "/orivis/api/agent/monitors"},
		{method: http.MethodPost, path: "/orivis/api/agent/results"},
	}

	for _, tt := range tests {
		if !server.Runtime().HasRoute(tt.method, tt.path) {
			t.Fatalf("expected route %s %s to be registered", tt.method, tt.path)
		}
	}

	for _, path := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, http.NoBody)
		rec := httptest.NewRecorder()
		server.Runtime().HumaAPI().Adapter().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected root health alias %s to return 200, got %d", path, rec.Code)
		}
	}
}
