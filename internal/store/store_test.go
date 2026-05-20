package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func TestOpenRejectsUnsupportedDriver(t *testing.T) {
	cfg := config.Config{}
	cfg.DB.Driver = "oracle"
	_, err := store.Open(cfg, testLogger())
	if err == nil {
		t.Fatal("expected unsupported driver error")
	}
}

func TestOpenRejectsLegacyPostgresAlias(t *testing.T) {
	cfg := config.Config{}
	cfg.DB.Driver = "postgres"
	cfg.DB.DSN = "postgres://127.0.0.1:5432/orivis?sslmode=disable"
	_, err := store.Open(cfg, testLogger())
	if err == nil {
		t.Fatal("expected legacy driver error for postgres")
	}
	if !strings.Contains(err.Error(), "supported drivers are sqlite, mysql, pgx") {
		t.Fatalf("expected supported drivers guidance in error, got %v", err)
	}
}

func TestOpenRunsSQLiteMigrations(t *testing.T) {
	ctx := context.Background()
	storage := newTestStore(t)

	for _, table := range []string{"environments", "regions", "agents", "monitors", "probe_results", "notification_deliveries", "schema_migrations"} {
		assertSQLiteTableExists(ctx, t, storage, table)
	}
}

func TestAgentRegisterAndHeartbeat(t *testing.T) {
	storage := newTestStore(t)
	agent := registerTestAgent(t, storage, "agent-guangdong-01", []string{"prod", "staging"})
	assertRegisteredAgent(t, agent)

	seenAt := time.Now().UTC().Add(time.Minute)
	updated, err := storage.AgentStore().RecordHeartbeat(context.Background(), store.AgentHeartbeatParams{
		AgentID: agent.ID,
		Token:   "secret-token",
		Version: "next-version",
		SeenAt:  seenAt,
	})
	if err != nil {
		t.Fatalf("record heartbeat: %v", err)
	}
	if updated.Version != "next-version" {
		t.Fatalf("expected updated version, got %q", updated.Version)
	}
	if !updated.LastSeenAt.Equal(seenAt) {
		t.Fatalf("expected last_seen_at %s, got %s", seenAt, updated.LastSeenAt)
	}
}

func TestAgentRegisterIsIdempotentByName(t *testing.T) {
	storage := newTestStore(t)
	first := registerTestAgent(t, storage, "agent-idempotent-01", []string{"dev"})

	second, err := storage.AgentStore().Register(context.Background(), store.RegisterAgentParams{
		Name:             "agent-idempotent-01",
		Token:            "secret-token",
		RegionCode:       "local",
		EnvironmentCodes: []string{"prod", "staging"},
		RuntimeType:      "docker-compose",
		Version:          "next-version",
	})
	if err != nil {
		t.Fatalf("register existing agent: %v", err)
	}

	if second.ID != first.ID {
		t.Fatalf("expected existing agent id %q, got %q", first.ID, second.ID)
	}
	if second.RuntimeType != "docker-compose" || second.Version != "next-version" {
		t.Fatalf("expected existing agent to be updated, got %#v", second)
	}
	if second.EnvironmentIDs == nil || second.EnvironmentIDs.Len() != 2 {
		t.Fatalf("expected replaced environments, got %#v", second.EnvironmentIDs)
	}
}

func TestAgentHeartbeatRejectsWrongToken(t *testing.T) {
	storage := newTestStore(t)
	agent := registerTestAgent(t, storage, "agent-test-01", nil)

	_, err := storage.AgentStore().RecordHeartbeat(context.Background(), store.AgentHeartbeatParams{
		AgentID: agent.ID,
		Token:   "wrong-token",
	})
	if !errors.Is(err, store.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestMonitorTasksAndProbeResults(t *testing.T) {
	storage := newTestStore(t)
	agent := registerTestAgent(t, storage, "agent-task-01", []string{"dev"})
	monitor := createTestMonitor(t, storage, agent, "API health")
	assertAssignedTask(t, storage, agent.ID, monitor.ID)
	result := recordTestResult(t, storage, agent, monitor.ID)
	assertProbeResult(t, result, agent, monitor.ID)
}

func TestMonitorAssignmentIsIdempotent(t *testing.T) {
	storage := newTestStore(t)
	agent := registerTestAgent(t, storage, "agent-assign-01", []string{"dev"})
	monitor := createTestMonitor(t, storage, agent, "API health")

	monitorIDs := []string{monitor.ID, monitor.ID, "", "   "}
	if err := storage.MonitorStore().AssignMonitors(context.Background(), monitorIDs); err != nil {
		t.Fatalf("assign monitors first pass: %v", err)
	}
	firstOwner := queryMonitorOwnerAgentID(t, storage, monitor.ID)
	if firstOwner == "" {
		t.Fatal("expected monitor owner on first assignment")
	}
	if got := queryMonitorOwnerCount(t, storage, monitor.ID); got != 1 {
		t.Fatalf("expected one owner row after first assignment, got %d", got)
	}

	if err := storage.MonitorStore().AssignMonitors(context.Background(), monitorIDs); err != nil {
		t.Fatalf("assign monitors second pass: %v", err)
	}
	secondOwner := queryMonitorOwnerAgentID(t, storage, monitor.ID)
	if secondOwner != firstOwner {
		t.Fatalf("expected owner %q to remain unchanged, got %q", firstOwner, secondOwner)
	}
	if got := queryMonitorOwnerCount(t, storage, monitor.ID); got != 1 {
		t.Fatalf("expected one owner row after second assignment, got %d", got)
	}
}

func TestNotificationDeliveryHistory(t *testing.T) {
	storage := newTestStore(t)
	now := time.Now().UTC()
	if err := storage.RecordNotificationDelivery(context.Background(), store.NotificationDeliveryParams{
		Channel:      store.NotificationChannelWebhook,
		Event:        "monitor_alert",
		MonitorID:    "monitor-1",
		AgentID:      "agent-1",
		Status:       store.NotificationStatusFailed,
		Attempt:      1,
		MaxAttempts:  3,
		HTTPStatus:   500,
		Duration:     50 * time.Millisecond,
		ErrorMessage: "webhook notification returned HTTP 500",
		CheckedAt:    now,
		SentAt:       now,
	}); err != nil {
		t.Fatalf("record notification delivery: %v", err)
	}

	items, err := storage.DashboardNotifications(context.Background(), 10)
	if err != nil {
		t.Fatalf("list dashboard notifications: %v", err)
	}
	if len(items) != 1 || items[0].Status != store.NotificationStatusFailed || items[0].HTTPStatus != 500 {
		t.Fatalf("unexpected notification history: %#v", items)
	}
}
