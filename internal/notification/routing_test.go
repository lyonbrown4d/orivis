package notification_test

import (
	"context"
	"testing"

	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/notification"
	obsx "github.com/lyonbrown4d/orivis/internal/observability"
)

func TestWebhookNotificationRouteMatchingByMonitorAndGroup(t *testing.T) {
	monitorRecorder := newWebhookPayloadRecorder(t)
	groupRecorder := newWebhookPayloadRecorder(t)
	monitorServer := newWebhookTestServer(monitorRecorder)
	groupServer := newWebhookTestServer(groupRecorder)
	defer monitorServer.Close()
	defer groupServer.Close()

	bus := newNotificationTestBus(t)
	storage := notificationTestStore(t)
	agent := notificationTestRegisterAgent(t, storage, "route-agent", []string{"prod"})
	monitorAPI := notificationTestCreateMonitor(t, storage, agent, "API health", "api")
	notificationTestCreateMonitor(t, storage, agent, "DB health", "db")

	cfg := notificationTestConfig("")
	cfg.Notification.Webhook.URL = ""
	cfg.Notification.Webhook.Routes = []string{
		"name=monitor-route;url=" + monitorServer.URL + ";monitors=" + monitorAPI,
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

	publishProbeResults(t, bus, notificationTestResultForMonitor(monitorAPI, model.StatusDown))
	_ = monitorRecorder.waitEvents(t, 1)
	_ = groupRecorder.waitEvents(t, 1)

	publishProbeResults(t, bus, notificationTestResultForMonitor("db-monitor-id", model.StatusDown))
	monitorRecorder.expectNoMoreEvents(t, 200)
	groupRecorder.expectNoMoreEvents(t, 200)
}
