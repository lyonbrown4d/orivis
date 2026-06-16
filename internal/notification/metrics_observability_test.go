package notification_test

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/arcgolabs/observabilityx"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/model"
	"github.com/lyonbrown4d/orivis/internal/notification"
)

type testMetricCounter struct {
	name  string
	value int64
	mu    sync.Mutex
}

func (m *testMetricCounter) Add(_ context.Context, value int64, _ ...observabilityx.Attribute) {
	m.mu.Lock()
	m.value += value
	m.mu.Unlock()
}

type testObservability struct {
	logger   *slog.Logger
	mu       sync.Mutex
	counters map[string]*testMetricCounter
}

func newTestObservability(t *testing.T) *testObservability {
	t.Helper()
	return &testObservability{
		logger:   slog.Default(),
		counters: make(map[string]*testMetricCounter),
	}
}

func (o *testObservability) Counter(spec observabilityx.CounterSpec) observabilityx.Counter {
	o.mu.Lock()
	defer o.mu.Unlock()

	name := strings.TrimSpace(spec.Name)
	counter := &testMetricCounter{name: name}
	o.counters[name] = counter
	return counter
}

func (o *testObservability) counterValue(name string) int64 {
	o.mu.Lock()
	counter, ok := o.counters[name]
	o.mu.Unlock()
	if !ok {
		return 0
	}
	counter.mu.Lock()
	defer counter.mu.Unlock()
	return counter.value
}

func (o *testObservability) counterExists(name string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, ok := o.counters[name]
	return ok
}

func (o *testObservability) Logger() *slog.Logger {
	return o.logger
}

func (o *testObservability) StartSpan(ctx context.Context, _ string, _ ...observabilityx.Attribute) (context.Context, observabilityx.Span) {
	return ctx, testSpan{}
}

func (o *testObservability) UpDownCounter(_ observabilityx.UpDownCounterSpec) observabilityx.UpDownCounter {
	return testUpDownCounter{}
}

func (o *testObservability) Histogram(_ observabilityx.HistogramSpec) observabilityx.Histogram {
	return testHistogram{}
}

func (o *testObservability) Gauge(_ observabilityx.GaugeSpec) observabilityx.Gauge {
	return testGauge{}
}

type testSpan struct{}

func (testSpan) End() {}

func (testSpan) RecordError(error) {}

func (testSpan) SetAttributes(...observabilityx.Attribute) {}

type testUpDownCounter struct{}

func (testUpDownCounter) Add(context.Context, int64, ...observabilityx.Attribute) {}

type testHistogram struct{}

func (testHistogram) Record(context.Context, float64, ...observabilityx.Attribute) {}

type testGauge struct{}

func (testGauge) Set(context.Context, float64, ...observabilityx.Attribute) {}

func waitCounterValue(t *testing.T, getValue func() int64, message string) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("expected %s, got %d", message, getValue())
		case <-ticker.C:
			if getValue() > 0 {
				return
			}
		}
	}
}

func TestNotificationRouteObservabilityMatchesSanitizedRouteNames(t *testing.T) {
	defaultRecorder := newWebhookPayloadRecorder(t)
	namedRecorder := newWebhookPayloadRecorder(t)
	defaultServer := newWebhookTestServer(defaultRecorder)
	namedServer := newWebhookTestServer(namedRecorder)
	defer defaultServer.Close()
	defer namedServer.Close()

	bus := newNotificationTestBus(t)
	obs := newTestObservability(t)

	cfg := notificationTestConfig(defaultServer.URL)
	cfg.Notification.Webhook.Routes = []string{
		"name=Critical Edge!;url=" + namedServer.URL + ";monitors=monitor-1",
	}
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
	_ = defaultRecorder.waitPayloadEvent(t)
	_ = namedRecorder.waitPayloadEvent(t)

	waitCounterValue(t, func() int64 { return obs.counterValue("notification_webhook_route_webhook_matched_total") }, "default route matched metric increments")
	waitCounterValue(t, func() int64 { return obs.counterValue("notification_webhook_route_webhook_enqueued_total") }, "default route enqueued metric increments")
	waitCounterValue(t, func() int64 {
		return obs.counterValue("notification_webhook_route_webhook_critical_edge_matched_total")
	}, "named route matched metric increments")
	waitCounterValue(t, func() int64 {
		return obs.counterValue("notification_webhook_route_webhook_critical_edge_enqueued_total")
	}, "named route enqueued metric increments")
	if !obs.counterExists("notification_webhook_routes_unrouted_total") {
		t.Fatalf("expected unrouted counter to be initialized")
	}
}

func TestNotificationRouteObservabilityRecordsUnroutedResults(t *testing.T) {
	bus := newNotificationTestBus(t)
	obs := newTestObservability(t)

	cfg := notificationTestConfig("")
	cfg.Notification.Webhook.URL = ""
	cfg.Notification.Webhook.Routes = []string{"name=api-route;url=http://127.0.0.1:1;monitors=non-matching-monitor"}
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
	waitCounterValue(t, func() int64 { return obs.counterValue("notification_webhook_routes_unrouted_total") }, "unrouted metric increments")
	if got := obs.counterValue("notification_webhook_route_webhook_matched_total"); got != 0 {
		t.Fatalf("unexpected matched metric increments: %d", got)
	}
	if got := obs.counterValue("notification_webhook_delivery_queue_full_total"); got != 0 {
		t.Fatalf("unexpected queue full metric increments: %d", got)
	}
}

func TestNotificationRouteObservabilityRecordsMatchedAndUnmatchedRoutes(t *testing.T) {
	monitorRecorder := newWebhookPayloadRecorder(t)
	unusedRecorder := newWebhookPayloadRecorder(t)
	monitorServer := newWebhookTestServer(monitorRecorder)
	unusedServer := newWebhookTestServer(unusedRecorder)
	defer monitorServer.Close()
	defer unusedServer.Close()

	bus := newNotificationTestBus(t)
	obs := newTestObservability(t)
	storage := notificationTestStore(t)
	agent := notificationTestRegisterAgent(t, storage, "route-agent-match-miss", []string{"prod"})
	monitorID := notificationTestCreateMonitor(t, storage, agent, "API health", "api")

	cfg := notificationTestConfig("")
	cfg.Notification.Webhook.URL = ""
	cfg.Notification.Webhook.Routes = []string{
		"name=monitor-route;url=" + monitorServer.URL + ";monitors=" + monitorID,
		"name=group-route;url=" + unusedServer.URL + ";groups=missing-group",
	}
	manager, err := notification.NewManager(cfg, nil, bus, cachex.NewMemoryStore(), storage, obs)
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
	_ = monitorRecorder.waitPayloadEvent(t)
	unusedRecorder.expectNoMoreEvents(t, 200)

	waitCounterValue(t, func() int64 {
		return obs.counterValue("notification_webhook_route_webhook_monitor_route_matched_total")
	}, "matched route metric increments")
	waitCounterValue(t, func() int64 {
		return obs.counterValue("notification_webhook_route_webhook_group_route_not_matched_total")
	}, "unmatched route metric increments")
	if got := obs.counterValue("notification_webhook_route_webhook_group_route_enqueued_total"); got != 0 {
		t.Fatalf("expected no unmatched route enqueue, got %d", got)
	}
	if got := obs.counterValue("notification_webhook_routes_unrouted_total"); got != 0 {
		t.Fatalf("expected routed result, got unrouted %d", got)
	}
}
