package store

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/hashicorp/go-memdb"
	"github.com/lyonbrown4d/orivis/internal/model"
)

const (
	memoryTableAgents            = "agents"
	memoryTableRegions           = "regions"
	memoryTableEnvironments      = "environments"
	memoryTableMonitors          = "monitors"
	memoryTableAgentEnvironments = "agent_environments"
	memoryTableMonitorAgents     = "monitor_agents"
	memoryTableProbeResults      = "probe_results"

	memoryResultRetention = 24 * time.Hour
	memoryCleanupInterval = time.Minute
)

type memoryStore struct {
	db              *memdb.MemDB
	logger          *slog.Logger
	scheduler       *gocron.Scheduler
	resultRetention time.Duration
}

type memoryAgentStore struct {
	store *memoryStore
}

type memoryMonitorStore struct {
	store *memoryStore
}

type memoryResultStore struct {
	store *memoryStore
}

type memoryAgentEnvironment struct {
	ID            string
	AgentID       string
	EnvironmentID string
}

type memoryMonitorAgent struct {
	ID        string
	MonitorID string
	AgentID   string
}

func openMemoryStore(resultRetentionValue, cleanupIntervalValue string, logger *slog.Logger) (*Store, error) {
	database, err := memdb.NewMemDB(memorySchema())
	if err != nil {
		return nil, fmt.Errorf("create memory store: %w", err)
	}
	resultRetention, err := memoryDuration(resultRetentionValue, memoryResultRetention, "db.resultretention")
	if err != nil {
		return nil, err
	}
	cleanupInterval, err := memoryDuration(cleanupIntervalValue, memoryCleanupInterval, "db.cleanupinterval")
	if err != nil {
		return nil, err
	}

	scheduler := gocron.NewScheduler(time.UTC)
	memory := &memoryStore{
		db:              database,
		logger:          logger,
		scheduler:       scheduler,
		resultRetention: resultRetention,
	}
	if _, err := scheduler.Every(cleanupInterval).Do(func() {
		if err := memory.CleanupExpiredResults(); err != nil && logger != nil {
			logger.Warn("cleanup expired memory results failed", slog.Any("error", err))
		}
	}); err != nil {
		return nil, fmt.Errorf("schedule memory cleanup: %w", err)
	}
	scheduler.StartAsync()

	return &Store{
		memory:   memory,
		agents:   &memoryAgentStore{store: memory},
		monitors: &memoryMonitorStore{store: memory},
		results:  &memoryResultStore{store: memory},
	}, nil
}

func (s *memoryStore) Close() error {
	if s == nil || s.scheduler == nil {
		return nil
	}
	s.scheduler.Stop()
	return nil
}

func memoryDuration(value string, fallback time.Duration, name string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be a duration: %w", ErrInvalidInput, name, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("%w: %s must be positive", ErrInvalidInput, name)
	}
	return duration, nil
}

func (s *memoryStore) CleanupExpiredResults() error {
	if s == nil || s.resultRetention <= 0 {
		return nil
	}

	cutoff := time.Now().UTC().Add(-s.resultRetention)
	txn := s.db.Txn(true)
	defer txn.Abort()

	expired, err := memoryExpiredResults(txn, cutoff)
	if err != nil {
		return err
	}
	if err := deleteMemoryRows(txn, memoryTableProbeResults, expired, "expired memory probe result"); err != nil {
		return err
	}
	txn.Commit()
	return nil
}

func memoryExpiredResults(txn *memdb.Txn, cutoff time.Time) ([]any, error) {
	it, err := txn.Get(memoryTableProbeResults, "id")
	if err != nil {
		return nil, fmt.Errorf("list memory probe results: %w", err)
	}

	expired := make([]any, 0)
	for raw := it.Next(); raw != nil; raw = it.Next() {
		result, err := memoryValue[model.ProbeResult](raw, "probe result")
		if err != nil {
			return nil, err
		}
		if result.CreatedAt.Before(cutoff) {
			expired = append(expired, raw)
		}
	}
	return expired, nil
}

func deleteMemoryRows(txn *memdb.Txn, table string, rows []any, name string) error {
	for _, raw := range rows {
		if err := txn.Delete(table, raw); err != nil {
			return fmt.Errorf("delete %s: %w", name, err)
		}
	}
	return nil
}

func (s *memoryStore) memoryEnvironmentIDByCode(code string) (string, error) {
	txn := s.db.Txn(false)
	defer txn.Abort()

	raw, err := txn.First(memoryTableEnvironments, "code", normalizeCode(code))
	if err != nil {
		return "", fmt.Errorf("find memory environment by code: %w", err)
	}
	if raw == nil {
		return "", fmt.Errorf("%w: environment %s", ErrNotFound, code)
	}
	environment, err := memoryValue[model.Environment](raw, "environment")
	if err != nil {
		return "", err
	}
	return environment.ID, nil
}
