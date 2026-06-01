package notification_test

import (
	"context"
	"testing"

	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/notification"
	obsx "github.com/lyonbrown4d/orivis/internal/observability"
)

func TestWebhookNotificationRoutesWithDefaultAndNamedRoute(t *testing.T) {
	defaultRecorder := newWebhookPayloadRecorder(t)
	namedRecorder := newWebhookPayloadRecorder(t)
	defaultServer := newWebhookTestServer(defaultRecorder)
	namedServer := newWebhookTestServer(namedRecorder)
	defer defaultServer.Close()
	defer namedServer.Close()

	bus := newNotificationTestBus(t)
	cfg := notificationTestConfig(defaultServer.URL)
	cfg.Notification.Webhook.Routes = []string{
		"name=critical-edge;url=" + namedServer.URL + ";monitors=monitor-1",
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
	defaultEvent := defaultRecorder.waitPayloadEvent(t)
	namedEvent := namedRecorder.waitPayloadEvent(t)
	if defaultEvent.Channel != "webhook" {
		t.Fatalf("expected default channel name, got %q", defaultEvent.Channel)
	}
	if namedEvent.Channel != "webhook:critical-edge" {
		t.Fatalf("expected named channel name, got %q", namedEvent.Channel)
	}

	publishProbeResults(t, bus, notificationTestResultForMonitor("monitor-2", model.StatusDown))
	_ = defaultRecorder.waitEvents(t, 2)
	namedRecorder.expectNoMoreEvents(t, 200)
}

func TestWebhookNotificationRoutesByMonitorAndGroupOverlap(t *testing.T) {
	monitorRecorder := newWebhookPayloadRecorder(t)
	groupRecorder := newWebhookPayloadRecorder(t)
	monitorServer := newWebhookTestServer(monitorRecorder)
	groupServer := newWebhookTestServer(groupRecorder)
	defer monitorServer.Close()
	defer groupServer.Close()

	bus := newNotificationTestBus(t)
	storage := notificationTestStore(t)
	agent := notificationTestRegisterAgent(t, storage, "route-agent-overlap", []string{"prod"})
	monitorID := notificationTestCreateMonitor(t, storage, agent, "API health", "api")

	cfg := notificationTestConfig("")
	cfg.Notification.Webhook.URL = ""
	cfg.Notification.Webhook.Routes = []string{
		"name=monitor-route;url=" + monitorServer.URL + ";monitors=" + monitorID,
		"name=group-route;url=" + groupServer.URL + ";groups=api",
	}
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

	publishProbeResults(t, bus, notificationTestResultForMonitor(monitorID, model.StatusDown))
	monitorEvent := monitorRecorder.waitPayloadEvent(t)
	groupEvent := groupRecorder.waitPayloadEvent(t)
	if got, want := monitorEvent.Channel, "webhook:monitor-route"; got != want {
		t.Fatalf("expected monitor route channel %q, got %q", want, got)
	}
	if got, want := groupEvent.Channel, "webhook:group-route"; got != want {
		t.Fatalf("expected group route channel %q, got %q", want, got)
	}
}
