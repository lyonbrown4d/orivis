package notification

import (
	"context"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
)

func (m *Manager) webhookChannels(ctx context.Context, result model.ProbeResult) ([]webhookChannel, error) {
	if len(m.channels) == 0 {
		return nil, nil
	}
	groupName, err := m.monitorGroupName(ctx, result.MonitorID)
	if err != nil {
		return nil, err
	}
	return matchingWebhookChannels(m.channels, result.MonitorID, groupName), nil
}

func (m *Manager) monitorGroupName(ctx context.Context, monitorID string) (string, error) {
	if m.storage == nil || m.storage.MonitorStore() == nil {
		return "", nil
	}
	monitor, err := m.storage.MonitorStore().Get(ctx, monitorID)
	if err != nil {
		return "", wrapError(err, "load monitor for notification routing")
	}
	return monitor.GroupName, nil
}

func (m *Manager) enqueueWebhook(ctx context.Context, payload webhookPayload, channel webhookChannel) error {
	select {
	case <-ctx.Done():
		return wrapError(ctx.Err(), "enqueue webhook notification")
	case m.deliveries <- webhookDelivery{payload: payload, channel: channel}:
		m.metrics.observeWebhookRouteEnqueued(ctx, channel.channelName())
		return nil
	default:
		m.metrics.observeWebhookQueueFull(ctx)
		return newError("webhook notification delivery queue is full")
	}
}

func (m *Manager) enabled() bool {
	return len(m.channels) > 0
}

func newWebhookPayload(event string, result model.ProbeResult, sentAt time.Time) webhookPayload {
	return webhookPayload{
		Event:         event,
		MonitorID:     result.MonitorID,
		AgentID:       result.AgentID,
		RegionID:      result.RegionID,
		EnvironmentID: result.EnvironmentID,
		Status:        result.Status,
		LatencyMS:     result.Latency.Milliseconds(),
		ErrorMessage:  result.ErrorMessage,
		CheckedAt:     result.CheckedAt,
		SentAt:        sentAt,
	}
}
