package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func TestDashboardSnapshotResponseUsesMonitorNamesForResultsAndNotifications(t *testing.T) {
	storage := newAPITestStore(t)
	agent := registerDashboardTestAgent(t, storage, "agent-dashboard-idx-01", []string{"dev"})
	seed := seedDashboardSnapshotData(t, storage, agent)

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	server := newAPITestServer(cfg, storage)
	response := getJSON[struct {
		RecentResults []dashboardSnapshotResultItem       `json:"recent_results"`
		Notifications []dashboardSnapshotNotificationItem `json:"notifications"`
	}](t, server.Runtime().HumaAPI().Adapter(), "/api/dashboard/snapshot", http.StatusOK)

	resultNames := map[string]string{}
	for _, item := range response.RecentResults {
		resultNames[item.MonitorID] = item.MonitorName
	}
	assertDashboardSnapshotMonitorNames(t, response.RecentResults, response.Notifications, seed.API.ID, seed.Dashboard.ID)
}

type dashboardSnapshotResultItem struct {
	MonitorID   string `json:"monitor_id"`
	MonitorName string `json:"monitor_name"`
}

type dashboardSnapshotNotificationItem struct {
	MonitorName string `json:"monitor_name"`
}

func assertDashboardSnapshotMonitorNames(
	t *testing.T,
	results []dashboardSnapshotResultItem,
	notifications []dashboardSnapshotNotificationItem,
	apiMonitorID, dashboardMonitorID string,
) {
	t.Helper()

	if len(results) != 2 {
		t.Fatalf("expected 2 recent results, got %d", len(results))
	}
	resultNames := map[string]string{}
	for _, item := range results {
		resultNames[item.MonitorID] = item.MonitorName
	}
	if got, ok := resultNames[apiMonitorID]; !ok || got != "api" {
		t.Fatalf("expected api monitor name %q for result, got %q, ok=%v", "api", got, ok)
	}
	if got, ok := resultNames[dashboardMonitorID]; !ok || got != "dashboard" {
		t.Fatalf("expected dashboard monitor name %q for result, got %q, ok=%v", "dashboard", got, ok)
	}

	if len(notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications))
	}
	if got := notifications[0].MonitorName; got != "dashboard" {
		t.Fatalf("expected dashboard notification monitor name %q, got %q", "dashboard", got)
	}
}

type dashboardSnapshotSeed struct {
	API       model.Monitor
	Dashboard model.Monitor
}

func seedDashboardSnapshotData(t *testing.T, storage *store.Store, agent model.Agent) dashboardSnapshotSeed {
	t.Helper()
	environmentIDs := agent.EnvironmentIDs.Values()
	if len(environmentIDs) == 0 {
		t.Fatal("expected agent environment ids")
	}
	environmentID := environmentIDs[0]

	apiMonitor := createDashboardMonitor(t, storage, environmentID, "manual://api-monitor", "api", "https://example.com/api/health")
	dashboardMonitor := createDashboardMonitor(
		t,
		storage,
		environmentID,
		"manual://dashboard-monitor",
		"dashboard",
		"https://example.com/dashboard/health",
	)
	assignDashboardMonitorToAgent(t, storage, apiMonitor.ID, agent.ID)
	assignDashboardMonitorToAgent(t, storage, dashboardMonitor.ID, agent.ID)

	now := time.Now().UTC()
	recordDashboardResult(t, storage, agent, "result-api", apiMonitor.ID, model.StatusUp, now.Add(-time.Minute), 150*time.Millisecond)
	recordDashboardResult(t, storage, agent, "result-dashboard", dashboardMonitor.ID, model.StatusDown, now, 250*time.Millisecond)
	recordDashboardNotification(t, storage, dashboardMonitor.ID, agent, environmentID, now.Add(-30*time.Second))

	return dashboardSnapshotSeed{API: apiMonitor, Dashboard: dashboardMonitor}
}

func createDashboardMonitor(
	t *testing.T,
	storage *store.Store,
	environmentID, sourceKey, name, target string,
) model.Monitor {
	t.Helper()
	monitor, err := storage.MonitorStore().Create(context.Background(), store.CreateMonitorParams{
		SourceKey:         sourceKey,
		Name:              name,
		Type:              model.MonitorHTTP,
		Target:            target,
		EnvironmentID:     environmentID,
		Enabled:           true,
		Interval:          30 * time.Second,
		Timeout:           5 * time.Second,
		AggregationPolicy: model.AggregationMajorityDown,
	})
	if err != nil {
		t.Fatalf("create %s monitor: %v", name, err)
	}
	return monitor
}

func assignDashboardMonitorToAgent(t *testing.T, storage *store.Store, monitorID, agentID string) {
	t.Helper()
	if err := storage.MonitorStore().AssignAgent(context.Background(), monitorID, agentID); err != nil {
		t.Fatalf("assign monitor to agent: %v", err)
	}
}

func recordDashboardResult(
	t *testing.T,
	storage *store.Store,
	agent model.Agent,
	resultID, monitorID string,
	status model.Status,
	checkedAt time.Time,
	latency time.Duration,
) {
	t.Helper()
	if _, err := storage.ResultStore().Record(context.Background(), store.RecordProbeResultParams{
		Agent:     agent,
		ResultID:  resultID,
		MonitorID: monitorID,
		Status:    status,
		Latency:   latency,
		CheckedAt: checkedAt,
	}); err != nil {
		t.Fatalf("record result: %v", err)
	}
}

func recordDashboardNotification(
	t *testing.T,
	storage *store.Store,
	monitorID string,
	agent model.Agent,
	environmentID string,
	sentAt time.Time,
) {
	t.Helper()
	if err := storage.RecordNotificationDelivery(context.Background(), store.NotificationDeliveryParams{
		Channel:       store.NotificationChannelWebhook,
		Event:         "probe-failed",
		MonitorID:     monitorID,
		AgentID:       agent.ID,
		RegionID:      agent.RegionID,
		EnvironmentID: environmentID,
		Status:        store.NotificationStatusFailed,
		Attempt:       1,
		MaxAttempts:   3,
		HTTPStatus:    500,
		Duration:      12 * time.Millisecond,
		CheckedAt:     sentAt,
		SentAt:        sentAt.Add(30 * time.Second),
	}); err != nil {
		t.Fatalf("record notification: %v", err)
	}
}

func TestDashboardSnapshotResponseKeepsEmptyCollectionsAsArrays(t *testing.T) {
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	server := newAPITestServer(cfg, newAPITestStore(t))

	response := getJSON[struct {
		RecentResults []any `json:"recent_results"`
		Notifications []any `json:"notifications"`
	}](t, server.Runtime().HumaAPI().Adapter(), "/api/dashboard/snapshot", http.StatusOK)

	if response.RecentResults == nil || len(response.RecentResults) != 0 {
		t.Fatalf("expected empty recent_results array, got %#v", response.RecentResults)
	}
	if response.Notifications == nil || len(response.Notifications) != 0 {
		t.Fatalf("expected empty notifications array, got %#v", response.Notifications)
	}
}
