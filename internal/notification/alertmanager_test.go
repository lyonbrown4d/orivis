package notification_test

import (
	"context"
	"strings"
	"testing"

	"github.com/arcgolabs/eventx"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/notification"
	obsx "github.com/lyonbrown4d/orivis/internal/observability"
)

func TestWebhookNotificationAlertmanagerRouteType(t *testing.T) {
	payloads := newAlertmanagerPayloadRecorder(t)
	server := newWebhookTestServer(payloads)
	defer server.Close()

	bus := eventx.New()
	t.Cleanup(func() {
		if err := bus.Close(); err != nil {
			t.Errorf("close event bus: %v", err)
		}
	})

	cfg := notificationTestConfig("")
	cfg.Notification.Webhook.URL = ""
	cfg.Notification.Webhook.Routes = []string{
		"name=orivis-alerts;type=alertmanager;url=" + server.URL + ";monitors=monitor-1",
	}
	manager, err := notification.NewManager(cfg, nil, bus, cachex.NewMemoryStore(), nil, obsx.NewNop(nil))
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

	publishProbeResults(t, bus, notificationTestResultForMonitor("monitor-1", model.StatusDown))
	events := payloads.waitPayloads(t, 1)
	if events[0].Status != "firing" {
		t.Fatalf("expected firing alert, got %q", events[0].Status)
	}
	if events[0].Labels["status"] != "down" || events[0].Labels["event"] != "monitor_alert" {
		t.Fatalf("unexpected labels: %#v", events[0].Labels)
	}
	if events[0].Labels["channel"] != "alertmanager:orivis-alerts" {
		t.Fatalf("unexpected channel label: %q", events[0].Labels["channel"])
	}
}

func TestWebhookNotificationRouteRejectsUnsupportedType(t *testing.T) {
	bus := newNotificationTestBus(t)
	cfg := notificationTestConfig("")
	cfg.Notification.Webhook.URL = ""
	cfg.Notification.Webhook.Routes = []string{
		"name=unsupported;type=pagerduty;url=http://127.0.0.1:1;monitors=monitor-1",
	}
	_, err := notification.NewManager(cfg, nil, bus, cachex.NewMemoryStore(), nil, obsx.NewNop(nil))
	if err == nil {
		t.Fatal("expected unsupported route type error")
	}
}

func TestWebhookNotificationAlertmanagerResolvedStatus(t *testing.T) {
	payloads := newAlertmanagerPayloadRecorder(t)
	server := newWebhookTestServer(payloads)
	defer server.Close()

	bus := newNotificationTestBus(t)
	cfg := notificationTestConfig("")
	cfg.Notification.Webhook.URL = ""
	cfg.Notification.Webhook.Routes = []string{
		"name=orivis-alerts;type=alertmanager;url=" + server.URL + ";monitors=monitor-1",
	}
	manager, err := notification.NewManager(cfg, nil, bus, cachex.NewMemoryStore(), nil, obsx.NewNop(nil))
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

	publishProbeResults(t, bus, notificationTestResultForMonitor("monitor-1", model.StatusDown))
	_ = payloads.waitPayloads(t, 1)

	publishProbeResults(t, bus, notificationTestResultForMonitor("monitor-1", model.StatusUp))
	events := payloads.waitPayloads(t, 2)
	last := events[1]
	if last.Status != "resolved" {
		t.Fatalf("expected resolved status, got %q", last.Status)
	}
	if last.Labels["event"] != "monitor_recovered" || !strings.HasSuffix(last.Labels["resolved"], "true") {
		t.Fatalf("unexpected resolved payload labels: %#v", last.Labels)
	}
	if last.EndsAt == "" {
		t.Fatal("expected resolved payload to include endsAt timestamp")
	}
}
