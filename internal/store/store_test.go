package store

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/serverconfig"
)

func TestResolveDialectSQLite(t *testing.T) {
	dialect, driver, err := resolveDialect("sqlite")
	if err != nil {
		t.Fatalf("expected sqlite dialect: %v", err)
	}
	if driver != "sqlite" {
		t.Fatalf("expected sqlite driver, got %q", driver)
	}
	if dialect.Name() != "sqlite" {
		t.Fatalf("expected sqlite dialect, got %q", dialect.Name())
	}
}

func TestResolveDialectUnsupported(t *testing.T) {
	_, _, err := resolveDialect("postgres")
	if err == nil {
		t.Fatal("expected unsupported driver error")
	}
}

func TestOpenRunsSQLiteMigrations(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	for _, table := range []string{"environments", "regions", "agents", "monitors", "probe_results", "schema_migrations"} {
		var count int
		err := store.DB.QueryRowContext(
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
}

func TestOpenMemoryStoreSupportsWorkflow(t *testing.T) {
	ctx := context.Background()
	store := newTestMemoryStore(t)

	if store.DB != nil {
		t.Fatal("expected memory store to avoid opening a SQL database")
	}
	if store.memory == nil {
		t.Fatal("expected memory store backend")
	}

	agent, err := store.AgentStore().Register(ctx, RegisterAgentParams{
		Name:             "agent-memory-01",
		Token:            "secret-token",
		RegionCode:       "local",
		EnvironmentCodes: []string{"dev"},
		RuntimeType:      "host",
		Version:          "test-version",
	})
	if err != nil {
		t.Fatalf("register memory agent: %v", err)
	}
	environmentIDs := agent.EnvironmentIDs.Values()
	if len(environmentIDs) != 1 {
		t.Fatalf("expected one environment id, got %#v", environmentIDs)
	}

	monitor, err := store.MonitorStore().Create(ctx, CreateMonitorParams{
		Name:              "Memory API health",
		Type:              model.MonitorHTTP,
		Target:            "https://example.com/health",
		EnvironmentID:     environmentIDs[0],
		Enabled:           true,
		Interval:          30 * time.Second,
		Timeout:           5 * time.Second,
		AggregationPolicy: model.AggregationMajorityDown,
	})
	if err != nil {
		t.Fatalf("create memory monitor: %v", err)
	}
	if err := store.MonitorStore().AssignAgent(ctx, monitor.ID, agent.ID); err != nil {
		t.Fatalf("assign memory monitor: %v", err)
	}

	tasks, err := store.MonitorStore().ListAssignedEnabled(ctx, agent.ID)
	if err != nil {
		t.Fatalf("list memory monitor tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != monitor.ID {
		t.Fatalf("expected assigned memory monitor task, got %#v", tasks)
	}

	result, err := store.ResultStore().Record(ctx, RecordProbeResultParams{
		Agent:     agent,
		MonitorID: monitor.ID,
		Status:    model.StatusUp,
		Latency:   42 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("record memory result: %v", err)
	}
	if result.MonitorID != monitor.ID || result.AgentID != agent.ID || result.RegionID != agent.RegionID {
		t.Fatalf("unexpected memory result: %#v", result)
	}
}

func TestAgentRegisterAndHeartbeat(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	agent, err := store.AgentStore().Register(ctx, RegisterAgentParams{
		Name:             "agent-guangdong-01",
		Token:            "secret-token",
		RegionCode:       "Guangdong",
		EnvironmentCodes: []string{"prod", "staging"},
		RuntimeType:      "host",
		Version:          "test-version",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
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

	seenAt := time.Now().UTC().Add(time.Minute)
	updated, err := store.AgentStore().RecordHeartbeat(ctx, AgentHeartbeatParams{
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
	ctx := context.Background()
	store := newTestStore(t)

	agent, err := store.AgentStore().Register(ctx, RegisterAgentParams{
		Name:        "agent-test-01",
		Token:       "right-token",
		RegionCode:  "local",
		RuntimeType: "host",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}

	_, err = store.AgentStore().RecordHeartbeat(ctx, AgentHeartbeatParams{
		AgentID: agent.ID,
		Token:   "wrong-token",
	})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestMonitorTasksAndProbeResults(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	agent, err := store.AgentStore().Register(ctx, RegisterAgentParams{
		Name:             "agent-task-01",
		Token:            "secret-token",
		RegionCode:       "local",
		EnvironmentCodes: []string{"dev"},
		RuntimeType:      "host",
	})
	if err != nil {
		t.Fatalf("register agent: %v", err)
	}
	environmentIDs := agent.EnvironmentIDs.Values()
	if len(environmentIDs) != 1 {
		t.Fatalf("expected one environment id, got %#v", environmentIDs)
	}

	monitor, err := store.MonitorStore().Create(ctx, CreateMonitorParams{
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
	if err := store.MonitorStore().AssignAgent(ctx, monitor.ID, agent.ID); err != nil {
		t.Fatalf("assign monitor: %v", err)
	}

	tasks, err := store.MonitorStore().ListAssignedEnabled(ctx, agent.ID)
	if err != nil {
		t.Fatalf("list assigned monitors: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != monitor.ID {
		t.Fatalf("expected assigned monitor task, got %#v", tasks)
	}

	result, err := store.ResultStore().Record(ctx, RecordProbeResultParams{
		Agent:     agent,
		MonitorID: monitor.ID,
		Status:    model.StatusUp,
		Latency:   42 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("record result: %v", err)
	}
	if result.MonitorID != monitor.ID || result.AgentID != agent.ID || result.RegionID != agent.RegionID {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "orivis.db")) + "?mode=rwc"

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := Open(cfg, logger)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(context.Background()); err != nil {
			t.Fatalf("close store: %v", err)
		}
	})
	return store
}

func newTestMemoryStore(t *testing.T) *Store {
	t.Helper()

	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "memory"

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := Open(cfg, logger)
	if err != nil {
		t.Fatalf("open memory store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(context.Background()); err != nil {
			t.Fatalf("close memory store: %v", err)
		}
	})
	return store
}
