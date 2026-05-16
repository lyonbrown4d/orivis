package api_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/lyonbrown4d/orivis/internal/api"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func TestAgentRegisterAndHeartbeatAPI(t *testing.T) {
	ctx := context.Background()
	storage := newAPITestStore(t)
	handler := newAgentAPIHandler(storage)

	registerResp := postJSON[protocol.AgentRegisterResponse](t, handler, "/api/agent/register", protocol.AgentRegisterRequest{
		Name:             "agent-api-01",
		Token:            "agent-token",
		RegionCode:       "local",
		EnvironmentCodes: []string{"dev"},
		RuntimeType:      "host",
		Version:          "test",
	}, http.StatusOK)
	if registerResp.AgentID == "" || registerResp.RegionID == "" {
		t.Fatalf("expected agent and region ids, got %#v", registerResp)
	}

	heartbeatResp := postJSON[protocol.AgentHeartbeatResponse](t, handler, "/api/agent/heartbeat", protocol.AgentHeartbeatRequest{
		AgentID: registerResp.AgentID,
		Token:   "agent-token",
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
	handler := newAgentAPIHandler(storage)

	agent := registerAPIAgent(t, handler, storage, "agent-api-task-01")
	monitor := createAPIMonitor(t, storage, agent)
	assertAPITasks(t, handler, agent.ID, monitor.ID)

	postJSON[map[string]string](t, handler, "/api/agent/results", protocol.AgentResultRequest{
		AgentID:   agent.ID,
		Token:     "agent-token",
		MonitorID: monitor.ID,
		Status:    string(model.StatusUp),
		LatencyMS: 35,
		CheckedAt: time.Now().UTC(),
	}, http.StatusOK)

	var count int
	err := storage.DB.QueryRowContext(ctx, "SELECT COUNT(1) FROM probe_results WHERE monitor_id = ? AND agent_id = ?", monitor.ID, agent.ID).Scan(&count)
	if err != nil {
		t.Fatalf("count probe results: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one probe result, got %d", count)
	}
}

func TestAgentMonitorSyncAPI(t *testing.T) {
	ctx := context.Background()
	storage := newAPITestStore(t)
	handler := newAgentAPIHandler(storage)

	registerResp := postJSON[protocol.AgentRegisterResponse](t, handler, "/api/agent/register", protocol.AgentRegisterRequest{
		Name:             "agent-sync-01",
		Token:            "agent-token",
		RegionCode:       "local",
		EnvironmentCodes: []string{"dev"},
		RuntimeType:      "docker",
	}, http.StatusOK)

	syncResp := postJSON[protocol.AgentMonitorSyncResponse](t, handler, "/api/agent/monitors", syncRequest(registerResp.AgentID, "http://web:8080/health", 15), http.StatusOK)
	if syncResp.Synced != 1 {
		t.Fatalf("expected one synced monitor, got %d", syncResp.Synced)
	}

	tasks := getJSON[protocol.AgentTasksResponse](t, handler, agentTasksPath(registerResp.AgentID), http.StatusOK)
	if len(tasks.Tasks) != 1 {
		t.Fatalf("expected one synced task, got %#v", tasks.Tasks)
	}
	if tasks.Tasks[0].Target != "http://web:8080/health" || tasks.Tasks[0].IntervalSeconds != 15 {
		t.Fatalf("unexpected synced task: %#v", tasks.Tasks[0])
	}

	postJSON[protocol.AgentMonitorSyncResponse](t, handler, "/api/agent/monitors", syncRequest(registerResp.AgentID, "http://web:8081/health", 30), http.StatusOK)

	var count int
	err := storage.DB.QueryRowContext(ctx, "SELECT COUNT(1) FROM monitors WHERE source_key = ?", "docker:container:web:http").Scan(&count)
	if err != nil {
		t.Fatalf("count discovered monitors: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected discovered monitor to be upserted, got %d rows", count)
	}
}

func TestAgentRegisterRejectsInvalidBootstrapToken(t *testing.T) {
	server := api.NewServer(agentAPITestConfig(), testLogger(), newAPITestStore(t), nil, nil)
	handler := server.Runtime().HumaAPI().Adapter()

	postJSON[map[string]any](t, handler, "/api/agent/register", protocol.AgentRegisterRequest{
		Name:        "agent-api-02",
		Token:       "wrong-token",
		RegionCode:  "local",
		RuntimeType: "host",
	}, http.StatusUnauthorized)
}

func newAgentAPIHandler(storage *store.Store) httpHandler {
	server := api.NewServer(agentAPITestConfig(), testLogger(), storage, nil, nil)
	return server.Runtime().HumaAPI().Adapter()
}

func agentAPITestConfig() config.Config {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.Auth.Agent.Token = "agent-token"
	return cfg
}

func registerAPIAgent(t *testing.T, handler httpHandler, storage *store.Store, name string) model.Agent {
	t.Helper()
	registerResp := postJSON[protocol.AgentRegisterResponse](t, handler, "/api/agent/register", protocol.AgentRegisterRequest{
		Name:             name,
		Token:            "agent-token",
		RegionCode:       "local",
		EnvironmentCodes: []string{"dev"},
		RuntimeType:      "host",
	}, http.StatusOK)
	agent, err := storage.AgentStore().Get(context.Background(), registerResp.AgentID)
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	return agent
}

func createAPIMonitor(t *testing.T, storage *store.Store, agent model.Agent) model.Monitor {
	t.Helper()
	environmentIDs := agent.EnvironmentIDs.Values()
	if len(environmentIDs) != 1 {
		t.Fatalf("expected one environment id, got %#v", environmentIDs)
	}
	monitor, err := storage.MonitorStore().Create(context.Background(), store.CreateMonitorParams{
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
	if err := storage.MonitorStore().AssignAgent(context.Background(), monitor.ID, agent.ID); err != nil {
		t.Fatalf("assign monitor: %v", err)
	}
	return monitor
}

func assertAPITasks(t *testing.T, handler httpHandler, agentID, monitorID string) {
	t.Helper()
	tasks := getJSON[protocol.AgentTasksResponse](t, handler, agentTasksPath(agentID), http.StatusOK)
	if len(tasks.Tasks) != 1 {
		t.Fatalf("expected one task, got %#v", tasks.Tasks)
	}
	if tasks.Tasks[0].MonitorID != monitorID || tasks.Tasks[0].Type != string(model.MonitorHTTP) {
		t.Fatalf("unexpected task: %#v", tasks.Tasks[0])
	}
}

func syncRequest(agentID, target string, interval int) protocol.AgentMonitorSyncRequest {
	return protocol.AgentMonitorSyncRequest{
		AgentID: agentID,
		Token:   "agent-token",
		Monitors: []protocol.AgentDiscoveredMonitor{
			{
				SourceKey:         "docker:container:web:http",
				Name:              "web",
				Type:              string(model.MonitorHTTP),
				Target:            target,
				EnvironmentCode:   "dev",
				IntervalSeconds:   interval,
				TimeoutSeconds:    3,
				AggregationPolicy: string(model.AggregationMajorityDown),
			},
		},
	}
}

func agentTasksPath(agentID string) string {
	return "/api/agent/tasks?agent_id=" + url.QueryEscape(agentID) + "&token=agent-token"
}
