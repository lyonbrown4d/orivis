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
	if etag := rec.Header().Get("ETag"); etag == "" {
		t.Fatal("expected ETag header")
	}
}

func TestDashboardSnapshotSupportsETag(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"

	server := newAPITestServer(cfg, newAPITestStore(t))
	handler := server.Runtime().HumaAPI().Adapter()

	firstReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/dashboard/snapshot", http.NoBody)
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	etag := firstRec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected first response ETag")
	}

	secondReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/dashboard/snapshot", http.NoBody)
	secondReq.Header.Set("If-None-Match", etag)
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusNotModified {
		t.Fatalf("expected status 304, got %d: %s", secondRec.Code, secondRec.Body.String())
	}
	if secondRec.Body.Len() != 0 {
		t.Fatalf("expected empty 304 response body, got %q", secondRec.Body.String())
	}
}
