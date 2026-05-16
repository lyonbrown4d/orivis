package api_test

import (
	"net/http"
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
