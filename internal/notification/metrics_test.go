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
	defaultEvents := defaultRecorder.waitEvents(t, 1)
	namedEvents := namedRecorder.waitEvents(t, 1)
	if got, want := len(defaultEvents), 1; got != want {
		t.Fatalf("expected %d default route events, got %d", want, got)
	}
	if got, want := len(namedEvents), 1; got != want {
		t.Fatalf("expected %d named route events, got %d", want, got)
	}

	publishProbeResults(t, bus, notificationTestResultForMonitor("monitor-2", model.StatusDown))
	_ = defaultRecorder.waitEvents(t, 2)
	namedRecorder.expectNoMoreEvents(t, 200)
}
