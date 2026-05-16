package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

func TestDashboardSnapshotReturnsJSON(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"

	server := newAPITestServer(cfg, newAPITestStore(t))
	handler := server.Runtime().HumaAPI().Adapter()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/dashboard/snapshot", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("expected application/json content type, got %q", got)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON body: %v", err)
	}
	if body["env"] != "test" {
		t.Fatalf("expected env test, got %#v", body["env"])
	}
}
