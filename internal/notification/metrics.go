package notification

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"unicode"

	"github.com/arcgolabs/observabilityx"
)

type notificationMetrics struct {
	obs                      observabilityx.Observability
	mu                       sync.Mutex
	routeMatchedByChannel    map[string]observabilityx.Counter
	routeEnqueuedByChannel   map[string]observabilityx.Counter
	routeNotMatchedByChannel map[string]observabilityx.Counter
	deliverySuccessByChannel map[string]observabilityx.Counter
	deliveryFailureByChannel map[string]observabilityx.Counter
	unrouted                 observabilityx.Counter
	queueFull                observabilityx.Counter
}

func newNotificationMetrics(obs observabilityx.Observability, logger *slog.Logger) notificationMetrics {
	obs = observabilityx.Normalize(obs, logger)
	return notificationMetrics{
		obs:                      obs,
		routeMatchedByChannel:    make(map[string]observabilityx.Counter),
		routeEnqueuedByChannel:   make(map[string]observabilityx.Counter),
		routeNotMatchedByChannel: make(map[string]observabilityx.Counter),
		deliverySuccessByChannel: make(map[string]observabilityx.Counter),
		deliveryFailureByChannel: make(map[string]observabilityx.Counter),
		unrouted: obs.Counter(observabilityx.NewCounterSpec(
			"notification_webhook_routes_unrouted_total",
			observabilityx.WithDescription("Total number of probe results with no matching webhook route."),
			observabilityx.WithUnit("events"),
		)),
		queueFull: obs.Counter(observabilityx.NewCounterSpec(
			"notification_webhook_delivery_queue_full_total",
			observabilityx.WithDescription("Total number of times the notification delivery queue was full."),
			observabilityx.WithUnit("events"),
		)),
	}
}

func (m *notificationMetrics) observeWebhookRouteMatched(ctx context.Context, channelName string) {
	m.observeRouteCounter(ctx, m.routeMatchedByChannel, channelName, "matched", "notification_webhook_route_%s_matched_total", "Total webhook deliveries routed to the channel.")
}

func (m *notificationMetrics) observeWebhookRouteNotMatched(ctx context.Context, channelName string) {
	m.observeRouteCounter(ctx, m.routeNotMatchedByChannel, channelName, "not_matched", "notification_webhook_route_%s_not_matched_total", "Total probe results that did not match this webhook route.")
}

func (m *notificationMetrics) observeWebhookRouteEnqueued(ctx context.Context, channelName string) {
	m.observeRouteCounter(ctx, m.routeEnqueuedByChannel, channelName, "enqueued", "notification_webhook_route_%s_enqueued_total", "Total webhook payloads enqueued for the channel.")
}

func (m *notificationMetrics) observeWebhookDeliverySuccess(ctx context.Context, channelName string) {
	m.observeRouteCounter(ctx, m.deliverySuccessByChannel, channelName, "delivery_success", "notification_webhook_route_%s_delivery_success_total", "Total webhook payloads delivered successfully for the channel.")
}

func (m *notificationMetrics) observeWebhookDeliveryFailure(ctx context.Context, channelName string) {
	m.observeRouteCounter(ctx, m.deliveryFailureByChannel, channelName, "delivery_failure", "notification_webhook_route_%s_delivery_failure_total", "Total webhook delivery failures for the channel.")
}

func (m *notificationMetrics) observeWebhookUnrouted(ctx context.Context) {
	m.unrouted.Add(ctx, 1)
}

func (m *notificationMetrics) observeWebhookQueueFull(ctx context.Context) {
	m.queueFull.Add(ctx, 1)
}

func (m *notificationMetrics) observeRouteCounter(
	ctx context.Context,
	counters map[string]observabilityx.Counter,
	channelName,
	suffix,
	nameTemplate,
	description string,
) {
	if m == nil || counters == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	name := metricRouteName(channelName)
	key := name + ":" + suffix
	counter, ok := counters[key]
	if !ok {
		counter = m.obs.Counter(observabilityx.NewCounterSpec(
			fmt.Sprintf(nameTemplate, name),
			observabilityx.WithDescription(description),
			observabilityx.WithUnit("events"),
		))
		counters[key] = counter
	}
	counter.Add(ctx, 1)
}

func metricRouteName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "webhook"
	}
	out := strings.Trim(
		strings.Map(func(runeValue rune) rune {
			switch {
			case unicode.IsLetter(runeValue), unicode.IsDigit(runeValue):
				return runeValue
			default:
				return '_'
			}
		}, value),
		"_",
	)
	if out == "" {
		return "webhook"
	}
	return out
}
