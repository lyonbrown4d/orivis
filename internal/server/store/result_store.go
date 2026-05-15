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

type ResultStore interface {
	Record(ctx context.Context, params RecordProbeResultParams) (model.ProbeResult, error)
}

type RecordProbeResultParams struct {
	Agent        model.Agent
	MonitorID    string
	Status       model.Status
	Latency      time.Duration
	ErrorMessage string
	CheckedAt    time.Time
	RawDetail    []byte
}

type resultStore struct {
	db *dbx.DB
}

func (s *resultStore) Record(ctx context.Context, params RecordProbeResultParams) (model.ProbeResult, error) {
	normalized, err := normalizeProbeResultParams(params)
	if err != nil {
		return model.ProbeResult{}, err
	}

	monitor, err := s.monitorForAgent(ctx, normalized.MonitorID, normalized.Agent.ID)
	if err != nil {
		return model.ProbeResult{}, err
	}

	id, err := newID("res")
	if err != nil {
		return model.ProbeResult{}, err
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO probe_results (
            id, monitor_id, agent_id, region_id, environment_id, status,
            latency_ms, error_message, checked_at, raw_detail, created_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		monitor.ID,
		normalized.Agent.ID,
		normalized.Agent.RegionID,
		monitor.EnvironmentID,
		string(normalized.Status),
		normalized.Latency.Milliseconds(),
		normalized.ErrorMessage,
		formatTime(normalized.CheckedAt),
		normalized.RawDetail,
		formatTime(now),
	); err != nil {
		return model.ProbeResult{}, fmt.Errorf("insert probe result: %w", err)
	}

	return model.ProbeResult{
		ID:            id,
		MonitorID:     monitor.ID,
		AgentID:       normalized.Agent.ID,
		RegionID:      normalized.Agent.RegionID,
		EnvironmentID: monitor.EnvironmentID,
		Status:        normalized.Status,
		Latency:       normalized.Latency,
		ErrorMessage:  normalized.ErrorMessage,
		CheckedAt:     normalized.CheckedAt,
		RawDetail:     normalized.RawDetail,
		CreatedAt:     now,
	}, nil
}

func (s *resultStore) monitorForAgent(ctx context.Context, monitorID, agentID string) (model.Monitor, error) {
	var rec monitorRecord
	err := s.db.QueryRowContext(
		ctx,
		`SELECT m.id, m.name, m.type, m.target, m.environment_id, m.enabled,
                m.interval_seconds, m.timeout_seconds, m.retry_count,
                m.aggregation_policy, m.source, m.created_at, m.updated_at
         FROM monitors m
         JOIN monitor_agents ma ON ma.monitor_id = m.id
         WHERE m.id = ? AND ma.agent_id = ? AND m.enabled = 1`,
		monitorID,
		agentID,
	).Scan(
		&rec.ID,
		&rec.Name,
		&rec.Type,
		&rec.Target,
		&rec.EnvironmentID,
		&rec.Enabled,
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
			return model.Monitor{}, fmt.Errorf("%w: assigned monitor %s", ErrNotFound, monitorID)
		}
		return model.Monitor{}, fmt.Errorf("find monitor for agent: %w", err)
	}
	return rec.model()
}

type normalizedProbeResultParams struct {
	Agent        model.Agent
	MonitorID    string
	Status       model.Status
	Latency      time.Duration
	ErrorMessage string
	CheckedAt    time.Time
	RawDetail    []byte
}

func normalizeProbeResultParams(params RecordProbeResultParams) (normalizedProbeResultParams, error) {
	out := normalizedProbeResultParams{
		Agent:        params.Agent,
		MonitorID:    strings.TrimSpace(params.MonitorID),
		Status:       params.Status,
		Latency:      params.Latency,
		ErrorMessage: strings.TrimSpace(params.ErrorMessage),
		CheckedAt:    params.CheckedAt.UTC(),
		RawDetail:    params.RawDetail,
	}
	if out.CheckedAt.IsZero() {
		out.CheckedAt = time.Now().UTC()
	}
	if out.Latency < 0 {
		out.Latency = 0
	}

	switch {
	case out.Agent.ID == "":
		return out, fmt.Errorf("%w: agent is required", ErrInvalidInput)
	case out.MonitorID == "":
		return out, fmt.Errorf("%w: monitor id is required", ErrInvalidInput)
	case !validProbeStatus(out.Status):
		return out, fmt.Errorf("%w: invalid probe status %q", ErrInvalidInput, out.Status)
	default:
		return out, nil
	}
}

func validProbeStatus(status model.Status) bool {
	switch status {
	case model.StatusUp, model.StatusDown, model.StatusDegraded, model.StatusUnknown:
		return true
	default:
		return false
	}
}
