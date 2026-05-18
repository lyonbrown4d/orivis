package notification_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/arcgolabs/eventx"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/ingest"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/notification"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

func TestWebhookNotificationUsesCachedAlertState(t *testing.T) {
	payloads := newWebhookPayloadRecorder(t)
	server := httptest.NewServer(payloads)
	defer server.Close()

	bus := eventx.New()
	t.Cleanup(func() {
		if err := bus.Close(); err != nil {
			t.Errorf("close event bus: %v", err)
		}
	})

	cfg := notificationTestConfig(server.URL)
	manager, err := notification.NewManager(cfg, nil, bus, cachex.NewMemoryStore())
	if err != nil {
		t.Fatalf("new notification manager: %v", err)
	}
	ctx := context.Background()
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("start notification manager: %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(ctx); err != nil {
			t.Errorf("stop notification manager: %v", err)
		}
	})

	down := notificationTestResult(model.StatusDown)
	up := notificationTestResult(model.StatusUp)
	publishProbeResults(t, bus, down)
	_ = payloads.waitEvents(t, 1)
	publishProbeResults(t, bus, down)
	payloads.expectNoMoreEvents(t, 100*time.Millisecond)
	publishProbeResults(t, bus, up)

	events := payloads.waitEvents(t, 2)
	if events[0] != "monitor_alert" || events[1] != "monitor_recovered" {
		t.Fatalf("unexpected events: %#v", events)
	}
}

type webhookPayloadRecorder struct {
	t      *testing.T
	mu     sync.Mutex
	events []string
}

func newWebhookPayloadRecorder(t *testing.T) *webhookPayloadRecorder {
	t.Helper()
	return &webhookPayloadRecorder{t: t}
}

func (r *webhookPayloadRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer closeRequestBody(r.t, req)
	var payload struct {
		Event string `json:"event"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.mu.Lock()
	r.events = append(r.events, payload.Event)
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
	return append([]string(nil), r.events...)
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

func notificationTestResult(status model.Status) model.ProbeResult {
	return model.ProbeResult{
		MonitorID: "monitor-1",
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
