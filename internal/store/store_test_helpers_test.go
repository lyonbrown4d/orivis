package store_test

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

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

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}
