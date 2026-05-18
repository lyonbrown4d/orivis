package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

func (m *Manager) deliverWebhook(ctx context.Context, payload webhookPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, webhookMethod(m.cfg), strings.TrimSpace(m.cfg.Notification.Webhook.URL), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if headerErr := applyWebhookHeaders(req, m.cfg, body); headerErr != nil {
		return headerErr
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("deliver webhook notification: %w", err)
	}
	defer closeBody(resp.Body)
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("read webhook response body: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("webhook notification returned HTTP %d", resp.StatusCode)
	}
	if m.logger != nil {
		m.logger.Info("sent webhook notification", "event", payload.Event, "monitor_id", payload.MonitorID, "status", payload.Status)
	}
	return nil
}

func (m *Manager) deliverWithRetry(ctx context.Context, delivery webhookDelivery) error {
	var lastErr error
	for attempt := 1; attempt <= m.maxAttempts; attempt++ {
		if err := m.deliverWebhook(ctx, delivery.payload); err != nil {
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

func webhookMethod(cfg config.Config) string {
	method := strings.ToUpper(strings.TrimSpace(cfg.Notification.Webhook.Method))
	if method == "" {
		return http.MethodPost
	}
	return method
}

func applyWebhookHeaders(req *http.Request, cfg config.Config, body []byte) error {
	for key, value := range webhookHeaders(cfg) {
		req.Header.Set(key, value)
	}
	signature, err := webhookSignature(cfg.Notification.Webhook.Secret, body)
	if err != nil {
		return err
	}
	if signature != "" {
		req.Header.Set("X-Orivis-Signature", signature)
	}
	return nil
}

func webhookHeaders(cfg config.Config) map[string]string {
	headers := make(map[string]string)
	for _, entry := range webhookHeaderEntries(cfg.Notification.Webhook.Headers) {
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

func closeBody(body io.Closer) {
	if body == nil {
		return
	}
	if err := body.Close(); err != nil {
		return
	}
}
