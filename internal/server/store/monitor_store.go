package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arcgolabs/dbx"
	"github.com/lyonbrown4d/orivis/internal/shared/model"
)

type MonitorStore interface {
	Create(ctx context.Context, params CreateMonitorParams) (model.Monitor, error)
	UpsertDiscovered(ctx context.Context, params UpsertDiscoveredMonitorParams) (model.Monitor, error)
	AssignAgent(ctx context.Context, monitorID, agentID string) error
	ListAssignedEnabled(ctx context.Context, agentID string) ([]model.Monitor, error)
	Get(ctx context.Context, id string) (model.Monitor, error)
}

type CreateMonitorParams struct {
	SourceKey         string
	Name              string
	Type              model.MonitorType
	Target            string
	EnvironmentID     string
	Enabled           bool
	Interval          time.Duration
	Timeout           time.Duration
	RetryCount        int
	AggregationPolicy model.AggregationPolicy
	Source            model.ConfigSource
}

type UpsertDiscoveredMonitorParams struct {
	SourceKey         string
	Name              string
	Type              model.MonitorType
	Target            string
	EnvironmentID     string
	Enabled           bool
	Interval          time.Duration
	Timeout           time.Duration
	RetryCount        int
	AggregationPolicy model.AggregationPolicy
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
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(
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
	); err != nil {
		return model.Monitor{}, fmt.Errorf("insert monitor: %w", err)
	}
	return s.Get(ctx, id)
}

func (s *monitorStore) UpsertDiscovered(ctx context.Context, params UpsertDiscoveredMonitorParams) (model.Monitor, error) {
	createParams := CreateMonitorParams{
		SourceKey:         params.SourceKey,
		Name:              params.Name,
		Type:              params.Type,
		Target:            params.Target,
		EnvironmentID:     params.EnvironmentID,
		Enabled:           params.Enabled,
		Interval:          params.Interval,
		Timeout:           params.Timeout,
		RetryCount:        params.RetryCount,
		AggregationPolicy: params.AggregationPolicy,
		Source:            model.ConfigSourceAPI,
	}
	normalized, err := normalizeCreateMonitorParams(createParams)
	if err != nil {
		return model.Monitor{}, err
	}
	if normalized.SourceKey == "" {
		return model.Monitor{}, fmt.Errorf("%w: monitor source key is required", ErrInvalidInput)
	}

	var existingID string
	err = s.db.QueryRowContext(ctx, "SELECT id FROM monitors WHERE source_key = ?", normalized.SourceKey).Scan(&existingID)
	switch {
	case err == nil:
		if _, err := s.db.ExecContext(
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
			existingID,
		); err != nil {
			return model.Monitor{}, fmt.Errorf("update discovered monitor: %w", err)
		}
		return s.Get(ctx, existingID)
	case errors.Is(err, sql.ErrNoRows):
		return s.Create(ctx, createParams)
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

	if _, err := s.db.ExecContext(
		ctx,
		"INSERT OR IGNORE INTO monitor_agents (monitor_id, agent_id) VALUES (?, ?)",
		monitorID,
		agentID,
	); err != nil {
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
	defer rows.Close()

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

	var rec monitorRecord
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, type, target, environment_id, enabled,
                source_key, interval_seconds, timeout_seconds, retry_count,
                aggregation_policy, source, created_at, updated_at
         FROM monitors
         WHERE id = ?`,
		id,
	).Scan(
		&rec.ID,
		&rec.Name,
		&rec.Type,
		&rec.Target,
		&rec.EnvironmentID,
		&rec.Enabled,
		&rec.SourceKey,
		&rec.IntervalSeconds,
		&rec.TimeoutSeconds,
		&rec.RetryCount,
		&rec.AggregationPolicy,
		&rec.Source,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Monitor{}, fmt.Errorf("%w: monitor %s", ErrNotFound, id)
		}
		return model.Monitor{}, fmt.Errorf("get monitor: %w", err)
	}
	return rec.model()
}

type createMonitorParams struct {
	SourceKey         string
	Name              string
	Type              model.MonitorType
	Target            string
	EnvironmentID     string
	Enabled           bool
	Interval          time.Duration
	Timeout           time.Duration
	RetryCount        int
	AggregationPolicy model.AggregationPolicy
	Source            model.ConfigSource
}

func normalizeCreateMonitorParams(params CreateMonitorParams) (createMonitorParams, error) {
	out := createMonitorParams{
		SourceKey:         strings.TrimSpace(params.SourceKey),
		Name:              strings.TrimSpace(params.Name),
		Type:              params.Type,
		Target:            strings.TrimSpace(params.Target),
		EnvironmentID:     strings.TrimSpace(params.EnvironmentID),
		Enabled:           params.Enabled,
		Interval:          params.Interval,
		Timeout:           params.Timeout,
		RetryCount:        max(0, params.RetryCount),
		AggregationPolicy: params.AggregationPolicy,
		Source:            params.Source,
	}
	if out.Interval <= 0 {
		out.Interval = 30 * time.Second
	}
	if out.Timeout <= 0 {
		out.Timeout = 5 * time.Second
	}
	if out.AggregationPolicy == "" {
		out.AggregationPolicy = model.AggregationMajorityDown
	}
	if out.Source == "" {
		out.Source = model.ConfigSourceAPI
	}

	switch {
	case out.Name == "":
		return out, fmt.Errorf("%w: monitor name is required", ErrInvalidInput)
	case out.Type == "":
		return out, fmt.Errorf("%w: monitor type is required", ErrInvalidInput)
	case out.Target == "":
		return out, fmt.Errorf("%w: monitor target is required", ErrInvalidInput)
	case out.EnvironmentID == "":
		return out, fmt.Errorf("%w: environment id is required", ErrInvalidInput)
	default:
		return out, nil
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMonitor(row rowScanner) (model.Monitor, error) {
	var rec monitorRecord
	if err := row.Scan(
		&rec.ID,
		&rec.Name,
		&rec.Type,
		&rec.Target,
		&rec.EnvironmentID,
		&rec.Enabled,
		&rec.SourceKey,
		&rec.IntervalSeconds,
		&rec.TimeoutSeconds,
		&rec.RetryCount,
		&rec.AggregationPolicy,
		&rec.Source,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	); err != nil {
		return model.Monitor{}, fmt.Errorf("scan monitor: %w", err)
	}
	return rec.model()
}

type monitorRecord struct {
	ID                string
	SourceKey         string
	Name              string
	Type              string
	Target            string
	EnvironmentID     string
	Enabled           int
	IntervalSeconds   int
	TimeoutSeconds    int
	RetryCount        int
	AggregationPolicy string
	Source            string
	CreatedAt         string
	UpdatedAt         string
}

func (r monitorRecord) model() (model.Monitor, error) {
	createdAt, err := parseTime(r.CreatedAt)
	if err != nil {
		return model.Monitor{}, err
	}
	updatedAt, err := parseTime(r.UpdatedAt)
	if err != nil {
		return model.Monitor{}, err
	}
	return model.Monitor{
		ID:                r.ID,
		SourceKey:         r.SourceKey,
		Name:              r.Name,
		Type:              model.MonitorType(r.Type),
		Target:            r.Target,
		EnvironmentID:     r.EnvironmentID,
		Enabled:           r.Enabled == 1,
		Interval:          time.Duration(r.IntervalSeconds) * time.Second,
		Timeout:           time.Duration(r.TimeoutSeconds) * time.Second,
		RetryCount:        r.RetryCount,
		AggregationPolicy: model.AggregationPolicy(r.AggregationPolicy),
		Source:            model.ConfigSource(r.Source),
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
