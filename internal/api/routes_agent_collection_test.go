package api_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func TestAgentMonitorsSyncReturnsSyncedCountAndAssignsMonitors(t *testing.T) {
	ctx := context.Background()
	storage := newAPITestStore(t)
	handler := newAgentAPIHandler(storage)
	agent := registerAPIAgent(t, handler, storage, "agent-sync-01")

	req := syncRequest(agent.ID, "http://web:8080/health", 15)
	req.Monitors = append(req.Monitors, protocol.AgentDiscoveredMonitor{
		SourceKey:         "docker:container:api:http",
		Name:              "api",
		Type:              string(model.MonitorHTTP),
		Target:            "http://api:8081/health",
		EnvironmentCode:   "dev",
		Enabled:           req.Monitors[0].Enabled,
		IntervalSeconds:   10,
		TimeoutSeconds:    3,
		AggregationPolicy: string(model.AggregationMajorityDown),
	})
	syncResp := postJSON[protocol.AgentMonitorSyncResponse](t, handler, "/api/agent/monitors", req, http.StatusOK)
	if syncResp.Synced != 2 {
		t.Fatalf("expected 2 synced monitors, got %d", syncResp.Synced)
	}

	assignedMonitors, err := storage.MonitorStore().ListAssignedEnabled(ctx, agent.ID)
	if err != nil {
		t.Fatalf("list assigned monitors: %v", err)
	}
	if len(assignedMonitors) != 2 {
		t.Fatalf("expected 2 assigned monitors, got %d", len(assignedMonitors))
	}
}

func TestAgentMonitorsSyncRejectsEmptyInput(t *testing.T) {
	storage := newAPITestStore(t)
	handler := newAgentAPIHandler(storage)
	agent := registerAPIAgent(t, handler, storage, "agent-sync-empty-01")

	syncResp := postJSON[protocol.AgentMonitorSyncResponse](t, handler, "/api/agent/monitors", protocol.AgentMonitorSyncRequest{
		AgentID:  agent.ID,
		Token:    "agent-token",
		Monitors: []protocol.AgentDiscoveredMonitor{},
	}, http.StatusOK)
	if syncResp.Synced != 0 {
		t.Fatalf("expected 0 synced monitors, got %d", syncResp.Synced)
	}
}

func TestAgentMonitorsSyncRejectsInvalidEnvironment(t *testing.T) {
	storage := newAPITestStore(t)
	handler := newAgentAPIHandler(storage)
	agent := registerAPIAgent(t, handler, storage, "agent-sync-qa-reject")

	req := syncRequest(agent.ID, "http://invalid.local/health", 10)
	req.Monitors[0].EnvironmentCode = "qa"

	postJSON[map[string]any](t, handler, "/api/agent/monitors", req, http.StatusNotFound)

	assignedMonitors, err := storage.MonitorStore().ListAssignedEnabled(context.Background(), agent.ID)
	if err != nil {
		t.Fatalf("list assigned monitors: %v", err)
	}
	if len(assignedMonitors) != 0 {
		t.Fatalf("expected no assigned monitors after failed sync, got %d", len(assignedMonitors))
	}
}
