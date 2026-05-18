// Package notification delivers server-side alert and recovery notifications.
package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/arcgolabs/eventx"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/ingest"
	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
)

type Manager struct {
	cfg         config.Config
	logger      *slog.Logger
	bus         eventx.BusRuntime
	cache       cachex.Store
	client      *http.Client
	mu          sync.Mutex
	states      map[string]alertState
	unsubscribe func()
}

type alertState struct {
	Active     bool         `json:"active"`
	Status     model.Status `json:"status"`
	LastSentAt time.Time    `json:"last_sent_at"`
}

type webhookPayload struct {
	Event         string       `json:"event"`
	MonitorID     string       `json:"monitor_id"`
	AgentID       string       `json:"agent_id"`
	RegionID      string       `json:"region_id"`
	EnvironmentID string       `json:"environment_id"`
	Status        model.Status `json:"status"`
	LatencyMS     int64        `json:"latency_ms"`
	ErrorMessage  string       `json:"error_message,omitempty"`
	CheckedAt     time.Time    `json:"checked_at"`
	SentAt        time.Time    `json:"sent_at"`
}

func NewManager(cfg config.Config, logger *slog.Logger, bus eventx.BusRuntime, cacheStore cachex.Store) (*Manager, error) {
	timeout, err := parseDuration(cfg.Notification.Webhook.Timeout, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("parse notification webhook timeout: %w", err)
	}
	return &Manager{
		cfg:    cfg,
		logger: logger,
		bus:    bus,
		cache:  cacheStore,
		client: &http.Client{Timeout: timeout},
		states: make(map[string]alertState),
	}, nil
}

func (m *Manager) Start(context.Context) error {
	if m == nil || !m.enabled() {
		return nil
	}
	if m.bus == nil {
		return errors.New("notification event bus is not available")
	}
	unsubscribe, err := eventx.Subscribe[ingest.ProbeResultsRecordedEvent](m.bus, m.handleProbeResultsRecorded)
	if err != nil {
		return fmt.Errorf("subscribe probe results recorded event: %w", err)
	}
	m.unsubscribe = unsubscribe
	return nil
}

func (m *Manager) Stop(context.Context) error {
	if m == nil || m.unsubscribe == nil {
		return nil
	}
	m.unsubscribe()
	m.unsubscribe = nil
	return nil
}

func (m *Manager) handleProbeResultsRecorded(ctx context.Context, event ingest.ProbeResultsRecordedEvent) error {
	for index := range event.Results {
		result := event.Results[index]
		payload, ok, err := m.nextWebhookPayload(ctx, result)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := m.deliverWebhook(ctx, payload); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) nextWebhookPayload(ctx context.Context, result model.ProbeResult) (webhookPayload, bool, error) {
	now := time.Now().UTC()
	cooldown, err := parseDuration(m.cfg.Notification.Webhook.Cooldown, 5*time.Minute)
	if err != nil {
		return webhookPayload{}, false, fmt.Errorf("parse notification webhook cooldown: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state, err := m.loadAlertState(ctx, result.MonitorID)
	if err != nil {
		return webhookPayload{}, false, err
	}
	if result.Status == model.StatusUp {
		return m.nextRecoveryPayload(ctx, result, state, now, cooldown)
	}
	return m.nextAlertPayload(ctx, result, state, now, cooldown)
}

func (m *Manager) nextRecoveryPayload(
	ctx context.Context,
	result model.ProbeResult,
	state alertState,
	now time.Time,
	cooldown time.Duration,
) (webhookPayload, bool, error) {
	if !state.Active || !m.cfg.Notification.Webhook.RecoveryEnabled {
		if err := m.storeAlertState(ctx, result.MonitorID, alertState{Active: false, Status: result.Status}, cooldown); err != nil {
			return webhookPayload{}, false, err
		}
		return webhookPayload{}, false, nil
	}
	if err := m.storeAlertState(ctx, result.MonitorID, alertState{Active: false, Status: result.Status, LastSentAt: now}, cooldown); err != nil {
		return webhookPayload{}, false, err
	}
	return newWebhookPayload("monitor_recovered", result, now), true, nil
}

func (m *Manager) nextAlertPayload(
	ctx context.Context,
	result model.ProbeResult,
	state alertState,
	now time.Time,
	cooldown time.Duration,
) (webhookPayload, bool, error) {
	if state.Active && state.Status == result.Status && now.Sub(state.LastSentAt) < cooldown {
		return webhookPayload{}, false, nil
	}
	if err := m.storeAlertState(ctx, result.MonitorID, alertState{Active: true, Status: result.Status, LastSentAt: now}, cooldown); err != nil {
		return webhookPayload{}, false, err
	}
	return newWebhookPayload("monitor_alert", result, now), true, nil
}

func (m *Manager) loadAlertState(ctx context.Context, monitorID string) (alertState, error) {
	if m.cache == nil {
		return m.states[monitorID], nil
	}
	raw, ok, err := m.cache.Get(ctx, alertStateCacheKey(monitorID))
	if err != nil {
		return alertState{}, fmt.Errorf("load alert state: %w", err)
	}
	if !ok {
		return alertState{}, nil
	}
	var state alertState
	if err := json.Unmarshal(raw, &state); err != nil {
		return alertState{}, fmt.Errorf("decode alert state: %w", err)
	}
	return state, nil
}

func (m *Manager) storeAlertState(ctx context.Context, monitorID string, state alertState, cooldown time.Duration) error {
	if m.cache == nil {
		m.states[monitorID] = state
		return nil
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode alert state: %w", err)
	}
	if err := m.cache.Set(ctx, alertStateCacheKey(monitorID), raw, alertStateTTL(cooldown)); err != nil {
		return fmt.Errorf("store alert state: %w", err)
	}
	return nil
}

func alertStateCacheKey(monitorID string) string {
	return "notification:alert:" + strings.TrimSpace(monitorID)
}

func alertStateTTL(cooldown time.Duration) time.Duration {
	if cooldown < time.Hour {
		return 24 * time.Hour
	}
	return cooldown * 24
}

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

func (m *Manager) enabled() bool {
	return m.cfg.Notification.Webhook.Enabled && strings.TrimSpace(m.cfg.Notification.Webhook.URL) != ""
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

func webhookMethod(cfg config.Config) string {
	method := strings.ToUpper(strings.TrimSpace(cfg.Notification.Webhook.Method))
	if method == "" {
		return http.MethodPost
	}
	return method
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
