package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
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
	if _, ok := body["notifications"].([]any); !ok {
		t.Fatalf("expected notifications array, got %#v", body["notifications"])
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

func TestDashboardMonitorDetailReturnsJSON(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"

	storage := newAPITestStore(t)
	agent := registerDashboardTestAgent(t, storage, "agent-01", []string{"dev"})
	monitor := createDashboardTestMonitor(t, storage, agent)
	if _, err := storage.ResultStore().Record(context.Background(), store.RecordProbeResultParams{
		Agent:     agent,
		ResultID:  "result-1",
		MonitorID: monitor.ID,
		Status:    model.StatusUp,
		Latency:   120 * time.Millisecond,
	}); err != nil {
		t.Fatalf("record result: %v", err)
	}
	if err := storage.RecordNotificationDelivery(context.Background(), store.NotificationDeliveryParams{
		Channel:      store.NotificationChannelWebhook,
		Event:        "monitor-updated",
		MonitorID:    monitor.ID,
		AgentID:      agent.ID,
		Status:       store.NotificationStatusSuccess,
		Attempt:      1,
		MaxAttempts:  3,
		HTTPStatus:   200,
		Duration:     42 * time.Millisecond,
		ErrorMessage: "",
		CheckedAt:    time.Now().UTC(),
		SentAt:       time.Now().UTC(),
	}); err != nil {
		t.Fatalf("record notification: %v", err)
	}

	server := newAPITestServer(cfg, storage)
	handler := server.Runtime().HumaAPI().Adapter()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/dashboard/monitor/"+monitor.ID+"?results=10&notifications=10", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Monitor       map[string]any   `json:"monitor"`
		Results       []map[string]any `json:"results"`
		Notifications []map[string]any `json:"notifications"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("expected JSON body: %v", err)
	}
	if got := out.Monitor["id"]; got != monitor.ID {
		t.Fatalf("expected monitor id %q, got %q", monitor.ID, got)
	}
	if got, want := len(out.Results), 1; got != want {
		t.Fatalf("expected %d results, got %d", want, got)
	}
	if got, want := len(out.Notifications), 1; got != want {
		t.Fatalf("expected %d notifications, got %d", want, got)
	}
}

func TestDashboardMonitorDetailReturnsNotFound(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"

	server := newAPITestServer(cfg, newAPITestStore(t))
	handler := server.Runtime().HumaAPI().Adapter()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/dashboard/monitor/does-not-exist", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func registerDashboardTestAgent(
	t *testing.T,
	storage *store.Store,
	name string,
	environments []string,
) model.Agent {
	t.Helper()
	agent, err := storage.AgentStore().Register(context.Background(), store.RegisterAgentParams{
		Name:             name,
		Token:            "secret-token",
		RegionCode:       "local",
		EnvironmentCodes: environments,
		RuntimeType:      "host",
		Version:          "test-version",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
	return agent
}

func createDashboardTestMonitor(t *testing.T, storage *store.Store, agent model.Agent) model.Monitor {
	t.Helper()
	environmentIDs := agent.EnvironmentIDs.Values()
	if len(environmentIDs) == 0 {
		t.Fatal("expected agent environment")
	}
	monitor, err := storage.MonitorStore().Create(context.Background(), store.CreateMonitorParams{
		Name:              "API health",
		Type:              model.MonitorHTTP,
		Target:            "https://example.com/health",
		EnvironmentID:     environmentIDs[0],
		Enabled:           true,
		Interval:          30 * time.Second,
		Timeout:           5 * time.Second,
		AggregationPolicy: model.AggregationMajorityDown,
	})
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	if err := storage.MonitorStore().AssignAgent(context.Background(), monitor.ID, agent.ID); err != nil {
		t.Fatalf("assign monitor: %v", err)
	}
	return monitor
}
