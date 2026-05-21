package notification

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/lyonbrown4d/orivis/internal/model"
)

func (m *Manager) nextWebhookPayload(ctx context.Context, result model.ProbeResult) (webhookPayload, bool, error) {
	now := time.Now().UTC()
	cooldown, err := parseDuration(m.cfg.Notification.Webhook.Cooldown, 5*time.Minute)
	if err != nil {
		return webhookPayload{}, false, wrapError(err, "parse notification webhook cooldown")
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
		return alertState{}, wrapError(err, "load alert state")
	}
	if !ok {
		return alertState{}, nil
	}
	var state alertState
	if err := json.Unmarshal(raw, &state); err != nil {
		return alertState{}, wrapError(err, "decode alert state")
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
		return wrapError(err, "encode alert state")
	}
	if err := m.cache.Set(ctx, alertStateCacheKey(monitorID), raw, alertStateTTL(cooldown)); err != nil {
		return wrapError(err, "store alert state")
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
