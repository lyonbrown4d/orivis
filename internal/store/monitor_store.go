package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arcgolabs/dbx"
	"github.com/lyonbrown4d/orivis/internal/model"
)

type MonitorStore interface {
	Create(ctx context.Context, params CreateMonitorParams) (model.Monitor, error)
	UpsertDiscovered(ctx context.Context, params UpsertDiscoveredMonitorParams) (model.Monitor, error)
	AssignAgent(ctx context.Context, monitorID, agentID string) error
	ListAssignedEnabled(ctx context.Context, agentID string) ([]model.Monitor, error)
	Get(ctx context.Context, id string) (model.Monitor, error)
}

type monitorStore struct {
	db *dbx.DB
}

func (s *monitorStore) Create(ctx context.Context, params CreateMonitorParams) (model.Monitor, error) {
	normalized, err := normalizeCreateMonitorParams(params)
	if err != nil {
		return model.Monitor{}, err
	}

	id, err := newID("mon")
	if err != nil {
		return model.Monitor{}, err
	}
	if err := s.insertMonitor(ctx, id, normalized); err != nil {
		return model.Monitor{}, err
	}
	return s.Get(ctx, id)
}

func (s *monitorStore) UpsertDiscovered(ctx context.Context, params UpsertDiscoveredMonitorParams) (model.Monitor, error) {
	normalized, err := normalizeDiscoveredMonitorParams(params)
	if err != nil {
		return model.Monitor{}, err
	}

	var existingID string
	err = s.db.QueryRowContext(ctx, "SELECT id FROM monitors WHERE source_key = ?", normalized.SourceKey).Scan(&existingID)
	switch {
	case err == nil:
		if updateErr := s.updateDiscoveredMonitor(ctx, existingID, normalized); updateErr != nil {
			return model.Monitor{}, updateErr
		}
		return s.Get(ctx, existingID)
	case errors.Is(err, sql.ErrNoRows):
		return s.Create(ctx, createMonitorParamsToPublic(normalized))
	default:
		return model.Monitor{}, fmt.Errorf("find discovered monitor: %w", err)
	}
}

func (s *monitorStore) AssignAgent(ctx context.Context, monitorID, agentID string) error {
	monitorID = strings.TrimSpace(monitorID)
	agentID = strings.TrimSpace(agentID)
	if monitorID == "" {
		return fmt.Errorf("%w: monitor id is required", ErrInvalidInput)
	}
	if agentID == "" {
		return fmt.Errorf("%w: agent id is required", ErrInvalidInput)
	}

	_, err := s.db.ExecContext(
		ctx,
		"INSERT OR IGNORE INTO monitor_agents (monitor_id, agent_id) VALUES (?, ?)",
		monitorID,
		agentID,
	)
	if err != nil {
		return fmt.Errorf("assign monitor agent: %w", err)
	}
	return nil
}

func (s *monitorStore) ListAssignedEnabled(ctx context.Context, agentID string) ([]model.Monitor, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("%w: agent id is required", ErrInvalidInput)
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT m.id, m.name, m.type, m.target, m.environment_id, m.enabled,
                m.source_key, m.interval_seconds, m.timeout_seconds, m.retry_count,
                m.aggregation_policy, m.source, m.created_at, m.updated_at
         FROM monitors m
         JOIN monitor_agents ma ON ma.monitor_id = m.id
         WHERE ma.agent_id = ? AND m.enabled = 1
         ORDER BY m.name`,
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list assigned monitors: %w", err)
	}
	defer closeRows(rows)

	monitors := make([]model.Monitor, 0)
	for rows.Next() {
		monitor, err := scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, monitor)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monitors: %w", err)
	}
	return monitors, nil
}

func (s *monitorStore) Get(ctx context.Context, id string) (model.Monitor, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return model.Monitor{}, fmt.Errorf("%w: monitor id is required", ErrInvalidInput)
	}

	monitor, err := s.getMonitor(ctx, id)
	if err != nil {
		return model.Monitor{}, err
	}
	return monitor, nil
}

func (s *monitorStore) insertMonitor(ctx context.Context, id string, normalized createMonitorParams) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO monitors (
            id, source_key, name, type, target, environment_id, enabled, interval_seconds,
            timeout_seconds, retry_count, aggregation_policy, source, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		normalized.SourceKey,
		normalized.Name,
		string(normalized.Type),
		normalized.Target,
		normalized.EnvironmentID,
		boolInt(normalized.Enabled),
		int(normalized.Interval.Seconds()),
		int(normalized.Timeout.Seconds()),
		normalized.RetryCount,
		string(normalized.AggregationPolicy),
		string(normalized.Source),
		formatTime(now),
		formatTime(now),
	)
	if err != nil {
		return fmt.Errorf("insert monitor: %w", err)
	}
	return nil
}

func (s *monitorStore) updateDiscoveredMonitor(ctx context.Context, id string, normalized createMonitorParams) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE monitors
             SET name = ?, type = ?, target = ?, environment_id = ?, enabled = ?,
                 interval_seconds = ?, timeout_seconds = ?, retry_count = ?,
                 aggregation_policy = ?, source = ?, updated_at = ?
             WHERE id = ?`,
		normalized.Name,
		string(normalized.Type),
		normalized.Target,
		normalized.EnvironmentID,
		boolInt(normalized.Enabled),
		int(normalized.Interval.Seconds()),
		int(normalized.Timeout.Seconds()),
		normalized.RetryCount,
		string(normalized.AggregationPolicy),
		string(normalized.Source),
		formatTime(time.Now().UTC()),
		id,
	)
	if err != nil {
		return fmt.Errorf("update discovered monitor: %w", err)
	}
	return nil
}

func (s *monitorStore) getMonitor(ctx context.Context, id string) (model.Monitor, error) {
	monitor, err := scanMonitor(s.db.QueryRowContext(
		ctx,
		`SELECT id, name, type, target, environment_id, enabled,
                source_key, interval_seconds, timeout_seconds, retry_count,
                aggregation_policy, source, created_at, updated_at
         FROM monitors
         WHERE id = ?`,
		id,
	))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Monitor{}, fmt.Errorf("%w: monitor %s", ErrNotFound, id)
		}
		return model.Monitor{}, fmt.Errorf("get monitor: %w", err)
	}
	return monitor, nil
}
