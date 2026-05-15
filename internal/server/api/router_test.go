package api

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/lyonbrown4d/orivis/internal/server/config"
)

func TestRoutesAreRegistered(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	server := NewServer(cfg, slog.Default(), nil, nil, nil)

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/"},
		{method: http.MethodGet, path: "/healthz"},
		{method: http.MethodGet, path: "/readyz"},
		{method: http.MethodGet, path: "/api/agent/tasks"},
		{method: http.MethodPost, path: "/api/agent/results"},
	}

	for _, tt := range tests {
		if !server.Runtime().HasRoute(tt.method, tt.path) {
			t.Fatalf("expected route %s %s to be registered", tt.method, tt.path)
		}
	}
}
