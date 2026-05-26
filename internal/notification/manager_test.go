package notification_test

import (
	"context"
	"testing"
	"time"

	"github.com/arcgolabs/eventx"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/notification"
	obsx "github.com/lyonbrown4d/orivis/internal/observability"
)

func TestWebhookNotificationUsesCachedAlertState(t *testing.T) {
	payloads := newWebhookPayloadRecorder(t)
	server := newWebhookTestServer(payloads)
	defer server.Close()

	bus := eventx.New()
	t.Cleanup(func() {
		if err := bus.Close(); err != nil {
			t.Errorf("close event bus: %v", err)
		}
	})

	cfg := notificationTestConfig(server.URL)
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

func TestWebhookNotificationRoutesByMonitor(t *testing.T) {
	payloads := newWebhookPayloadRecorder(t)
	server := newWebhookTestServer(payloads)
	defer server.Close()

	bus := newNotificationTestBus(t)
	cfg := notificationTestConfig("")
	cfg.Notification.Webhook.Routes = []string{"name=ops;url=" + server.URL + ";monitors=monitor-1"}
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
	_ = payloads.waitEvents(t, 1)
	publishProbeResults(t, bus, notificationTestResultForMonitor("monitor-2", model.StatusDown))
	payloads.expectNoMoreEvents(t, 100*time.Millisecond)
}

func TestWebhookNotificationRoutesByGroup(t *testing.T) {
	payloads := newWebhookPayloadRecorder(t)
	server := newWebhookTestServer(payloads)
	defer server.Close()

	bus := newNotificationTestBus(t)
	storage := notificationTestStore(t)
	agent := notificationTestRegisterAgent(t, storage, "agent-route-group", []string{"prod"})
	monitorAPI := notificationTestCreateMonitor(t, storage, agent, "API health", "api")
	monitorDB := notificationTestCreateMonitor(t, storage, agent, "DB health", "db")

	cfg := notificationTestConfig("")
	cfg.Notification.Webhook.URL = ""
	cfg.Notification.Webhook.Routes = []string{"name=api-route;url=" + server.URL + ";groups=api"}
	manager, err := notification.NewManager(cfg, nil, bus, cachex.NewMemoryStore(), storage, obsx.NewNop(nil))
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

	publishProbeResults(t, bus, notificationTestResultForMonitor(monitorAPI, model.StatusDown))
	_ = payloads.waitEvents(t, 1)
	publishProbeResults(t, bus, notificationTestResultForMonitor(monitorDB, model.StatusDown))
	payloads.expectNoMoreEvents(t, 100*time.Millisecond)
}
