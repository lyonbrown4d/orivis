package notification_test

import (
	"context"
	"testing"

	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/notification"
)

func TestWebhookNotificationMetricRouteNameNormalization(t *testing.T) {
	testCases := []struct {
		name     string
		rawRoute string
		expected string
	}{
		{
			name:     "empty uses fallback",
			rawRoute: "",
			expected: "webhook",
		},
		{
			name:     "spaces trimmed",
			rawRoute: "  ",
			expected: "webhook",
		},
		{
			name:     "upper-case normalized",
			rawRoute: "Critical Edge",
			expected: "webhook_critical_edge",
		},
		{
			name:     "special characters sanitized",
			rawRoute: "api/edge:prod",
			expected: "webhook_api_edge_prod",
		},
		{
			name:     "non-alnum becomes fallback",
			rawRoute: "###",
			expected: "webhook",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runWebhookRouteNormalizationCase(t, tc.rawRoute, tc.expected)
		})
	}
}

func runWebhookRouteNormalizationCase(t *testing.T, rawRoute, expected string) {
	t.Helper()

	recorder := newWebhookPayloadRecorder(t)
	server := newWebhookTestServer(recorder)
	defer server.Close()

	bus := newNotificationTestBus(t)
	obs := newTestObservability(t)

	cfg := notificationTestConfig(server.URL)
	cfg.Notification.Webhook.Routes = []string{"name=" + rawRoute + ";url=" + server.URL + ";monitors=monitor-1"}
	cfg.Notification.Webhook.Method = "POST"
	manager, err := notification.NewManager(cfg, nil, bus, cachex.NewMemoryStore(), nil, obs)
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
	_ = recorder.waitPayloadEvent(t)

	waitCounterValue(t, func() int64 {
		return obs.counterValue("notification_webhook_route_" + expected + "_matched_total")
	}, "named route matched metric increments")
}
