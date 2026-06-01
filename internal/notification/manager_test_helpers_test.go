package notification_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arcgolabs/eventx"
	"github.com/lyonbrown4d/orivis/internal/ingest"
	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

func newWebhookTestServer(handler http.Handler) *httptest.Server {
	return httptest.NewServer(handler)
}

func newNotificationTestBus(t *testing.T) eventx.BusRuntime {
	t.Helper()
	bus := eventx.New()
	t.Cleanup(func() {
		if err := bus.Close(); err != nil {
			t.Errorf("close event bus: %v", err)
		}
	})
	return bus
}

type webhookPayloadRecorder struct {
	t      *testing.T
	mu     sync.Mutex
	events []webhookPayloadEvent
}

type webhookPayloadEvent struct {
	Event   string `json:"event"`
	Channel string `json:"channel"`
}

func newWebhookPayloadRecorder(t *testing.T) *webhookPayloadRecorder {
	t.Helper()
	return &webhookPayloadRecorder{t: t}
}

func (r *webhookPayloadRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer closeRequestBody(r.t, req)
	var payload webhookPayloadEvent
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	r.events = append(r.events, payload)
	r.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (r *webhookPayloadRecorder) waitEvents(t *testing.T, count int) []string {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("expected %d webhook events, got %#v", count, r.snapshot())
		case <-ticker.C:
			if events := r.snapshot(); len(events) >= count {
				return events
			}
		}
	}
}

func (r *webhookPayloadRecorder) expectNoMoreEvents(t *testing.T, duration time.Duration) {
	t.Helper()
	before := len(r.snapshot())
	timer := time.NewTimer(duration)
	defer timer.Stop()
	<-timer.C
	after := len(r.snapshot())
	if after != before {
		t.Fatalf("expected no extra webhook events, before=%d after=%d events=%#v", before, after, r.snapshot())
	}
}

func (r *webhookPayloadRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.events))
	for _, item := range r.events {
		out = append(out, item.Event)
	}
	return out
}

func (r *webhookPayloadRecorder) snapshotEvents() []webhookPayloadEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]webhookPayloadEvent(nil), r.events...)
}

func (r *webhookPayloadRecorder) waitPayloadEvent(t *testing.T) webhookPayloadEvent {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	count := 1
	for {
		select {
		case <-deadline:
			t.Fatalf("expected %d webhook payloads, got %#v", count, r.snapshotEvents())
		case <-ticker.C:
			if events := r.snapshotEvents(); len(events) >= count {
				return events[0]
			}
		}
	}
}

func notificationTestConfig(url string) config.Config {
	var cfg config.Config
	cfg.Notification.Webhook.Enabled = true
	cfg.Notification.Webhook.URL = url
	cfg.Notification.Webhook.Method = http.MethodPost
	cfg.Notification.Webhook.Timeout = "1s"
	cfg.Notification.Webhook.Cooldown = "1h"
	cfg.Notification.Webhook.RecoveryEnabled = true
	return cfg
}

func notificationTestStore(t *testing.T) *store.Store {
	t.Helper()
	cfg := config.Config{}
	cfg.App.Env = "test"
	cfg.DB.Driver = "sqlite"
	cfg.DB.DSN = "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "notification-store.db")) + "?mode=rwc"

	storage, err := store.Open(cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := storage.Close(context.Background()); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return storage
}

func notificationTestRegisterAgent(t *testing.T, storage *store.Store, name string, environments []string) model.Agent {
	t.Helper()
	if len(environments) == 0 {
		environments = []string{"prod"}
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
		t.Fatalf("register test agent: %v", err)
	}
	return agent
}

func notificationTestCreateMonitor(t *testing.T, storage *store.Store, agent model.Agent, name, group string) string {
	t.Helper()
	environmentIDs := agent.EnvironmentIDs.Values()
	if len(environmentIDs) != 1 {
		t.Fatalf("expected one environment for test agent, got %#v", environmentIDs)
	}
	target := "https://example.com/" + group + "/" + sanitizeMonitorName(name)
	monitor, err := storage.MonitorStore().Create(context.Background(), store.CreateMonitorParams{
		SourceKey:     fmt.Sprintf("orivis-test://%s/%s", group, sanitizeMonitorName(name)),
		Name:          name,
		Type:          model.MonitorHTTP,
		Target:        target,
		GroupName:     group,
		EnvironmentID: environmentIDs[0],
		Enabled:       true,
		Interval:      time.Second,
		Timeout:       time.Second,
	})
	if err != nil {
		t.Fatalf("create test monitor: %v", err)
	}
	if err := storage.MonitorStore().AssignAgent(context.Background(), monitor.ID, agent.ID); err != nil {
		t.Fatalf("assign monitor to test agent: %v", err)
	}
	return monitor.ID
}

func sanitizeMonitorName(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	switch {
	case key == "":
		return "untitled"
	case len(key) <= 16:
		return key
	default:
		return key[:16]
	}
}

func notificationTestResult(status model.Status) model.ProbeResult {
	return notificationTestResultForMonitor("monitor-1", status)
}

func notificationTestResultForMonitor(monitorID string, status model.Status) model.ProbeResult {
	return model.ProbeResult{
		MonitorID: monitorID,
		AgentID:   "agent-1",
		Status:    status,
		CheckedAt: time.Now().UTC(),
	}
}

func publishProbeResults(t *testing.T, bus eventx.BusRuntime, results ...model.ProbeResult) {
	t.Helper()
	if err := bus.PublishAsync(context.Background(), ingest.ProbeResultsRecordedEvent{Results: results}); err != nil {
		t.Fatalf("publish probe results: %v", err)
	}
}

func closeRequestBody(t *testing.T, req *http.Request) {
	t.Helper()
	if err := req.Body.Close(); err != nil {
		t.Errorf("close request body: %v", err)
	}
}
