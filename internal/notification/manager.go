// Package notification delivers server-side alert and recovery notifications.
package notification

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/arcgolabs/eventx"
	cachex "github.com/lyonbrown4d/orivis/internal/cache"
	"github.com/lyonbrown4d/orivis/internal/ingest"
	"github.com/lyonbrown4d/orivis/internal/model"
	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/lyonbrown4d/orivis/internal/store"
)

type Manager struct {
	cfg           config.Config
	logger        *slog.Logger
	bus           eventx.BusRuntime
	cache         cachex.Store
	storage       *store.Store
	client        *http.Client
	mu            sync.Mutex
	states        map[string]alertState
	deliveries    chan webhookDelivery
	stop          chan struct{}
	done          chan struct{}
	stopOnce      sync.Once
	unsubscribe   func()
	channels      []webhookChannel
	maxAttempts   int
	retryInterval time.Duration
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

type webhookDelivery struct {
	payload webhookPayload
	channel webhookChannel
}

func NewManager(cfg config.Config, logger *slog.Logger, bus eventx.BusRuntime, cacheStore cachex.Store, storage *store.Store) (*Manager, error) {
	channels, err := webhookChannelsFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	timeout, err := parseDuration(cfg.Notification.Webhook.Timeout, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("parse notification webhook timeout: %w", err)
	}
	retryInterval, err := parseDuration(cfg.Notification.Webhook.RetryInterval, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("parse notification webhook retry interval: %w", err)
	}
	return &Manager{
		cfg:           cfg,
		logger:        logger,
		bus:           bus,
		cache:         cacheStore,
		storage:       storage,
		client:        &http.Client{Timeout: timeout},
		states:        make(map[string]alertState),
		deliveries:    make(chan webhookDelivery, webhookQueueSize(cfg)),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
		channels:      channels,
		maxAttempts:   webhookMaxAttempts(cfg),
		retryInterval: retryInterval,
	}, nil
}

func (m *Manager) Start(ctx context.Context) error {
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
	go m.run(ctx)
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if m.unsubscribe != nil {
		m.unsubscribe()
		m.unsubscribe = nil
	} else {
		return nil
	}
	m.stopOnce.Do(func() {
		close(m.stop)
	})
	<-m.done
	return nil
}

func (m *Manager) run(ctx context.Context) {
	defer close(m.done)
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stop:
			m.drainDeliveries(context.WithoutCancel(ctx))
			return
		case delivery := <-m.deliveries:
			m.logDeliveryError(m.deliverWithRetry(ctx, delivery))
		}
	}
}

func (m *Manager) drainDeliveries(ctx context.Context) {
	for {
		select {
		case delivery := <-m.deliveries:
			m.logDeliveryError(m.deliverWithRetry(ctx, delivery))
		default:
			return
		}
	}
}

func (m *Manager) handleProbeResultsRecorded(ctx context.Context, event ingest.ProbeResultsRecordedEvent) error {
	for index := range event.Results {
		if err := m.handleProbeResult(ctx, event.Results[index]); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) handleProbeResult(ctx context.Context, result model.ProbeResult) error {
	payload, ok, err := m.nextWebhookPayload(ctx, result)
	if err != nil || !ok {
		return err
	}
	channels, err := m.webhookChannels(ctx, result)
	if err != nil {
		return err
	}
	for index := range channels {
		if err := m.enqueueWebhook(ctx, payload, channels[index]); err != nil {
			return err
		}
	}
	return nil
}

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
		return "", fmt.Errorf("load monitor for notification routing: %w", err)
	}
	return monitor.GroupName, nil
}

func (m *Manager) enqueueWebhook(ctx context.Context, payload webhookPayload, channel webhookChannel) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("enqueue webhook notification: %w", ctx.Err())
	case m.deliveries <- webhookDelivery{payload: payload, channel: channel}:
		return nil
	default:
		return errors.New("webhook notification delivery queue is full")
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
