package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
	"resty.dev/v3"
)

func (m *Manager) deliverWebhook(ctx context.Context, delivery webhookDelivery) (int, error) {
	payload := delivery.payload
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal webhook payload: %w", err)
	}
	req := m.client.R().
		SetBody(body).
		SetHeader("Content-Type", "application/json")
	if headerErr := applyWebhookHeaders(req, delivery.channel, body); headerErr != nil {
		return 0, headerErr
	}

	resp, err := m.client.Execute(ctx, req, delivery.channel.method, strings.TrimSpace(delivery.channel.url))
	if err != nil {
		return 0, fmt.Errorf("deliver webhook notification: %w", err)
	}
	if resp.StatusCode() >= http.StatusBadRequest {
		return resp.StatusCode(), fmt.Errorf("webhook notification returned HTTP %d", resp.StatusCode())
	}
	if m.logger != nil {
		m.logger.Info("sent webhook notification", "channel", delivery.channel.channelName(), "event", payload.Event, "monitor_id", payload.MonitorID, "status", payload.Status)
	}
	return resp.StatusCode(), nil
}

func (m *Manager) deliverWithRetry(ctx context.Context, delivery webhookDelivery) error {
	var lastErr error
	for attempt := 1; attempt <= m.maxAttempts; attempt++ {
		startedAt := time.Now()
		statusCode, err := m.deliverWebhook(ctx, delivery)
		m.recordDeliveryAttempt(ctx, delivery, attempt, statusCode, time.Since(startedAt), err)
		if err != nil {
			lastErr = err
			if !m.waitBeforeRetry(ctx, attempt) {
				return fmt.Errorf("deliver webhook notification: %w", lastErr)
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("deliver webhook notification after %d attempts: %w", m.maxAttempts, lastErr)
}

func (m *Manager) recordDeliveryAttempt(
	ctx context.Context,
	delivery webhookDelivery,
	attempt int,
	statusCode int,
	duration time.Duration,
	deliveryErr error,
) {
	if m.storage == nil {
		return
	}
	status := store.NotificationStatusSuccess
	errMessage := ""
	if deliveryErr != nil {
		status = store.NotificationStatusFailed
		errMessage = deliveryErr.Error()
	}
	payload := delivery.payload
	if err := m.storage.RecordNotificationDelivery(context.WithoutCancel(ctx), store.NotificationDeliveryParams{
		Channel:       delivery.channel.channelName(),
		Event:         payload.Event,
		MonitorID:     payload.MonitorID,
		AgentID:       payload.AgentID,
		RegionID:      payload.RegionID,
		EnvironmentID: payload.EnvironmentID,
		Status:        status,
		Attempt:       attempt,
		MaxAttempts:   m.maxAttempts,
		HTTPStatus:    statusCode,
		Duration:      duration,
		ErrorMessage:  errMessage,
		CheckedAt:     payload.CheckedAt,
		SentAt:        payload.SentAt,
	}); err != nil && m.logger != nil {
		m.logger.Warn("record notification delivery failed", "error", err)
	}
}

func (m *Manager) waitBeforeRetry(ctx context.Context, attempt int) bool {
	if attempt >= m.maxAttempts {
		return false
	}
	timer := time.NewTimer(m.retryInterval * time.Duration(attempt))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-m.stop:
		return false
	case <-timer.C:
		return true
	}
}

func (m *Manager) logDeliveryError(err error) {
	if err != nil && m.logger != nil {
		m.logger.Error("deliver webhook notification", "error", err)
	}
}

func applyWebhookHeaders(req *resty.Request, channel webhookChannel, body []byte) error {
	for key, value := range webhookHeaders(channel.headers) {
		req.SetHeader(key, value)
	}
	signature, err := webhookSignature(channel.secret, body)
	if err != nil {
		return err
	}
	if signature != "" {
		req.SetHeader("X-Orivis-Signature", signature)
	}
	return nil
}

func webhookHeaders(values []string) map[string]string {
	headers := make(map[string]string)
	for _, entry := range webhookHeaderEntries(values) {
		key, value, ok := splitWebhookHeader(entry)
		if ok {
			headers[key] = value
		}
	}
	return headers
}

func webhookHeaderEntries(values []string) []string {
	entries := make([]string, 0, len(values))
	for _, value := range values {
		for entry := range strings.SplitSeq(value, ",") {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

func splitWebhookHeader(entry string) (string, string, bool) {
	key, value, ok := strings.Cut(entry, "=")
	if !ok {
		key, value, ok = strings.Cut(entry, ":")
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	return key, value, ok && key != ""
}

func webhookSignature(secret string, body []byte) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", nil
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write(body); err != nil {
		return "", fmt.Errorf("sign webhook payload: %w", err)
	}
	return "sha256=" + hex.EncodeToString(mac.Sum(nil)), nil
}

func webhookQueueSize(cfg config.Config) int {
	if cfg.Notification.Webhook.QueueSize <= 0 {
		return 128
	}
	return cfg.Notification.Webhook.QueueSize
}

func webhookMaxAttempts(cfg config.Config) int {
	if cfg.Notification.Webhook.MaxAttempts <= 0 {
		return 3
	}
	return cfg.Notification.Webhook.MaxAttempts
}

func parseDuration(value string, fallback time.Duration) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", value, err)
	}
	if duration <= 0 {
		return fallback, nil
	}
	return duration, nil
}
