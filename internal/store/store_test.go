package store_test

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func TestOpenRejectsUnsupportedDriver(t *testing.T) {
	cfg := config.Config{}
	cfg.DB.Driver = "postgres"
	_, err := store.Open(cfg, testLogger())
	if err == nil {
		t.Fatal("expected unsupported driver error")
	}
}

func TestOpenRunsSQLiteMigrations(t *testing.T) {
	ctx := context.Background()
	storage := newTestStore(t)

	for _, table := range []string{"environments", "regions", "agents", "monitors", "probe_results", "schema_migrations"} {
		assertSQLiteTableExists(ctx, t, storage, table)
	}
}

func TestOpenMemoryStoreSupportsWorkflow(t *testing.T) {
	storage := newTestMemoryStore(t)
	if storage.DB != nil {
		t.Fatal("expected memory store to avoid opening a SQL database")
	}

	agent := registerTestAgent(t, storage, "agent-memory-01", []string{"dev"})
	monitor := createTestMonitor(t, storage, agent, "Memory API health")
	assertAssignedTask(t, storage, agent.ID, monitor.ID)
	result := recordTestResult(t, storage, agent, monitor.ID)
	assertProbeResult(t, result, agent, monitor.ID)
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

func assertSQLiteTableExists(ctx context.Context, t *testing.T, storage *store.Store, table string) {
	t.Helper()
	var count int
	err := storage.DB.QueryRowContext(
		ctx,
		"SELECT COUNT(1) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query sqlite_master for %s: %v", table, err)
	}
	if count != 1 {
		t.Fatalf("expected table %s to exist", table)
	}
}

func registerTestAgent(t *testing.T, storage *store.Store, name string, environments []string) model.Agent {
	t.Helper()
	if environments == nil {
		environments = []string{"dev"}
	}
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

func assertRegisteredAgent(t *testing.T, agent model.Agent) {
	t.Helper()
	if agent.ID == "" {
		t.Fatal("expected agent id")
	}
	if agent.TokenHash == "" || agent.TokenHash == "secret-token" {
		t.Fatalf("expected stored token hash, got %q", agent.TokenHash)
	}
	if agent.Status != model.AgentStatusOnline {
		t.Fatalf("expected online agent, got %q", agent.Status)
	}
	if agent.EnvironmentIDs == nil || agent.EnvironmentIDs.Len() != 2 {
		t.Fatalf("expected 2 environment ids, got %#v", agent.EnvironmentIDs)
	}
}

func createTestMonitor(t *testing.T, storage *store.Store, agent model.Agent, name string) model.Monitor {
	t.Helper()
	environmentID := singleEnvironmentID(t, agent)
	monitor, err := storage.MonitorStore().Create(context.Background(), store.CreateMonitorParams{
		Name:              name,
		Type:              model.MonitorHTTP,
		Target:            "https://example.com/health",
		EnvironmentID:     environmentID,
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

func singleEnvironmentID(t *testing.T, agent model.Agent) string {
	t.Helper()
	environmentIDs := agent.EnvironmentIDs.Values()
	if len(environmentIDs) != 1 {
		t.Fatalf("expected one environment id, got %#v", environmentIDs)
	}
	return environmentIDs[0]
}

func assertAssignedTask(t *testing.T, storage *store.Store, agentID, monitorID string) {
	t.Helper()
	tasks, err := storage.MonitorStore().ListAssignedEnabled(context.Background(), agentID)
	if err != nil {
		t.Fatalf("list assigned monitors: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != monitorID {
		t.Fatalf("expected assigned monitor task, got %#v", tasks)
	}
}

func recordTestResult(t *testing.T, storage *store.Store, agent model.Agent, monitorID string) model.ProbeResult {
	t.Helper()
	result, err := storage.ResultStore().Record(context.Background(), store.RecordProbeResultParams{
		Agent:     agent,
		MonitorID: monitorID,
		Status:    model.StatusUp,
		Latency:   42 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("record result: %v", err)
	}
	return result
}

func assertProbeResult(t *testing.T, result model.ProbeResult, agent model.Agent, monitorID string) {
	t.Helper()
	if result.MonitorID != monitorID || result.AgentID != agent.ID || result.RegionID != agent.RegionID {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "orivis.db")) + "?mode=rwc"

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

func newTestMemoryStore(t *testing.T) *store.Store {
	t.Helper()

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "memory"

	storage, err := store.Open(cfg, testLogger())
	if err != nil {
		t.Fatalf("open memory store: %v", err)
	}
	t.Cleanup(func() {
		if err := storage.Close(context.Background()); err != nil {
			t.Fatalf("close memory store: %v", err)
		}
	})
	return storage
}

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
