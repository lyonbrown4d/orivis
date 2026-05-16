package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	"github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
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

func TestDashboardIndexRendersHTML(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "memory"

	server := NewServer(cfg, testLogger(), newAPITestStore(t), nil, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("expected text/html content type, got %q", got)
	}
	if !strings.Contains(rec.Body.String(), "Orivis") {
		t.Fatalf("expected dashboard body to contain Orivis, got %s", rec.Body.String())
	}
}

func TestAgentRegisterAndHeartbeatAPI(t *testing.T) {
	ctx := context.Background()
	storage := newAPITestStore(t)

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.Auth.Agent.Token = "bootstrap-token"

	server := NewServer(cfg, testLogger(), storage, nil, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	registerResp := postJSON[protocol.AgentRegisterResponse](t, handler, "/api/agent/register", protocol.AgentRegisterRequest{
		Name:             "agent-api-01",
		Token:            "bootstrap-token",
		RegionCode:       "local",
		EnvironmentCodes: []string{"dev"},
		RuntimeType:      "host",
		Version:          "test",
	}, http.StatusOK)

	if registerResp.AgentID == "" {
		t.Fatal("expected agent id")
	}
	if registerResp.RegionID == "" {
		t.Fatal("expected region id")
	}

	heartbeatResp := postJSON[protocol.AgentHeartbeatResponse](t, handler, "/api/agent/heartbeat", protocol.AgentHeartbeatRequest{
		AgentID: registerResp.AgentID,
		Token:   "bootstrap-token",
		Version: "next",
	}, http.StatusOK)
	if heartbeatResp.AgentID != registerResp.AgentID {
		t.Fatalf("expected heartbeat agent id %q, got %q", registerResp.AgentID, heartbeatResp.AgentID)
	}

	agent, err := storage.AgentStore().Get(ctx, registerResp.AgentID)
	if err != nil {
		t.Fatalf("get stored agent: %v", err)
	}
	if agent.Version != "next" {
		t.Fatalf("expected heartbeat to update version, got %q", agent.Version)
	}
}

func TestAgentTasksAndResultsAPI(t *testing.T) {
	ctx := context.Background()
	storage := newAPITestStore(t)

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.Auth.Agent.Token = "agent-token"

	server := NewServer(cfg, testLogger(), storage, nil, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	registerResp := postJSON[protocol.AgentRegisterResponse](t, handler, "/api/agent/register", protocol.AgentRegisterRequest{
		Name:             "agent-api-task-01",
		Token:            "agent-token",
		RegionCode:       "local",
		EnvironmentCodes: []string{"dev"},
		RuntimeType:      "host",
	}, http.StatusOK)

	agent, err := storage.AgentStore().Get(ctx, registerResp.AgentID)
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	environmentIDs := agent.EnvironmentIDs.Values()
	if len(environmentIDs) != 1 {
		t.Fatalf("expected one environment id, got %#v", environmentIDs)
	}
	monitor, err := storage.MonitorStore().Create(ctx, store.CreateMonitorParams{
		Name:              "API health",
		Type:              model.MonitorHTTP,
		Target:            "https://example.com/health",
		EnvironmentID:     environmentIDs[0],
		Enabled:           true,
		Timeout:           5 * time.Second,
		AggregationPolicy: model.AggregationMajorityDown,
	})
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	if err := storage.MonitorStore().AssignAgent(ctx, monitor.ID, agent.ID); err != nil {
		t.Fatalf("assign monitor: %v", err)
	}

	tasks := getJSON[protocol.AgentTasksResponse](
		t,
		handler,
		"/api/agent/tasks?agent_id="+url.QueryEscape(agent.ID)+"&token=agent-token",
		http.StatusOK,
	)
	if len(tasks.Tasks) != 1 {
		t.Fatalf("expected one task, got %#v", tasks.Tasks)
	}
	if tasks.Tasks[0].MonitorID != monitor.ID || tasks.Tasks[0].Type != string(model.MonitorHTTP) {
		t.Fatalf("unexpected task: %#v", tasks.Tasks[0])
	}

	postJSON[map[string]string](t, handler, "/api/agent/results", protocol.AgentResultRequest{
		AgentID:   agent.ID,
		Token:     "agent-token",
		MonitorID: monitor.ID,
		Status:    string(model.StatusUp),
		LatencyMS: 35,
		CheckedAt: time.Now().UTC(),
	}, http.StatusOK)

	var count int
	if err := storage.DB.QueryRowContext(ctx, "SELECT COUNT(1) FROM probe_results WHERE monitor_id = ? AND agent_id = ?", monitor.ID, agent.ID).Scan(&count); err != nil {
		t.Fatalf("count probe results: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one probe result, got %d", count)
	}
}

func TestAgentMonitorSyncAPI(t *testing.T) {
	ctx := context.Background()
	storage := newAPITestStore(t)

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.Auth.Agent.Token = "agent-token"

	server := NewServer(cfg, testLogger(), storage, nil, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	registerResp := postJSON[protocol.AgentRegisterResponse](t, handler, "/api/agent/register", protocol.AgentRegisterRequest{
		Name:             "agent-sync-01",
		Token:            "agent-token",
		RegionCode:       "local",
		EnvironmentCodes: []string{"dev"},
		RuntimeType:      "docker",
	}, http.StatusOK)

	syncResp := postJSON[protocol.AgentMonitorSyncResponse](t, handler, "/api/agent/monitors", protocol.AgentMonitorSyncRequest{
		AgentID: registerResp.AgentID,
		Token:   "agent-token",
		Monitors: []protocol.AgentDiscoveredMonitor{
			{
				SourceKey:         "docker:container:web:http",
				Name:              "web",
				Type:              string(model.MonitorHTTP),
				Target:            "http://web:8080/health",
				EnvironmentCode:   "dev",
				IntervalSeconds:   15,
				TimeoutSeconds:    3,
				AggregationPolicy: string(model.AggregationMajorityDown),
			},
		},
	}, http.StatusOK)
	if syncResp.Synced != 1 {
		t.Fatalf("expected one synced monitor, got %d", syncResp.Synced)
	}

	tasks := getJSON[protocol.AgentTasksResponse](
		t,
		handler,
		"/api/agent/tasks?agent_id="+url.QueryEscape(registerResp.AgentID)+"&token=agent-token",
		http.StatusOK,
	)
	if len(tasks.Tasks) != 1 {
		t.Fatalf("expected one synced task, got %#v", tasks.Tasks)
	}
	if tasks.Tasks[0].Target != "http://web:8080/health" || tasks.Tasks[0].IntervalSeconds != 15 {
		t.Fatalf("unexpected synced task: %#v", tasks.Tasks[0])
	}

	postJSON[protocol.AgentMonitorSyncResponse](t, handler, "/api/agent/monitors", protocol.AgentMonitorSyncRequest{
		AgentID: registerResp.AgentID,
		Token:   "agent-token",
		Monitors: []protocol.AgentDiscoveredMonitor{
			{
				SourceKey:         "docker:container:web:http",
				Name:              "web",
				Type:              string(model.MonitorHTTP),
				Target:            "http://web:8081/health",
				EnvironmentCode:   "dev",
				IntervalSeconds:   30,
				TimeoutSeconds:    5,
				AggregationPolicy: string(model.AggregationMajorityDown),
			},
		},
	}, http.StatusOK)

	var count int
	if err := storage.DB.QueryRowContext(ctx, "SELECT COUNT(1) FROM monitors WHERE source_key = ?", "docker:container:web:http").Scan(&count); err != nil {
		t.Fatalf("count discovered monitors: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected discovered monitor to be upserted, got %d rows", count)
	}
}

func TestAgentRegisterRejectsInvalidBootstrapToken(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.Auth.Agent.Token = "bootstrap-token"

	server := NewServer(cfg, testLogger(), newAPITestStore(t), nil, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	postJSON[map[string]any](t, handler, "/api/agent/register", protocol.AgentRegisterRequest{
		Name:        "agent-api-02",
		Token:       "wrong-token",
		RegionCode:  "local",
		RuntimeType: "host",
	}, http.StatusUnauthorized)
}

type httpHandler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

func getJSON[T any](t *testing.T, handler httpHandler, path string, expectedStatus int) T {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, rec.Code, rec.Body.String())
	}

	var out T
	if rec.Body.Len() == 0 {
		return out
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}
	return out
}

func postJSON[T any](t *testing.T, handler httpHandler, path string, body any, expectedStatus int) T {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d: %s", expectedStatus, rec.Code, rec.Body.String())
	}

	var out T
	if rec.Body.Len() == 0 {
		return out
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v; body=%s", err, rec.Body.String())
	}
	return out
}

func newAPITestStore(t *testing.T) *store.Store {
	t.Helper()

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "orivis-api.db")) + "?mode=rwc"

	storage, err := store.Open(cfg, testLogger())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := storage.Close(context.Background()); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})
	return storage
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
