package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arcgolabs/dbx"
	repository "github.com/arcgolabs/dbx/repository"
	"github.com/lyonbrown4d/orivis/internal/model"
)

type ResultStore interface {
	Record(ctx context.Context, params RecordProbeResultParams) (model.ProbeResult, error)
	RecordBatch(ctx context.Context, params []RecordProbeResultParams) ([]model.ProbeResult, error)
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

type resultQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *dbx.Row
}

func (s *resultStore) Record(ctx context.Context, params RecordProbeResultParams) (model.ProbeResult, error) {
	results, err := s.RecordBatch(ctx, []RecordProbeResultParams{params})
	if err != nil {
		return model.ProbeResult{}, err
	}
	if len(results) == 0 {
		return model.ProbeResult{}, fmt.Errorf("%w: result batch returned no records", ErrInvalidInput)
	}
	return results[0], nil
}

func (s *resultStore) RecordBatch(ctx context.Context, params []RecordProbeResultParams) ([]model.ProbeResult, error) {
	if len(params) == 0 {
		return nil, nil
	}

	var results []model.ProbeResult
	err := newProbeResultRepository(s.db).InTx(ctx, nil, func(tx *dbx.Tx, repo *repository.Base[probeResultRow, probeResultSchema]) error {
		nextResults, rows, err := s.prepareProbeResultRows(ctx, tx, params)
		if err != nil {
			return err
		}
		if err := repo.CreateMany(ctx, rows...); err != nil {
			return fmt.Errorf("create probe result batch: %w", err)
		}
		results = nextResults
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("record probe result batch: %w", err)
	}

	return results, nil
}

func (s *resultStore) prepareProbeResultRows(
	ctx context.Context,
	queryer resultQueryer,
	params []RecordProbeResultParams,
) ([]model.ProbeResult, []*probeResultRow, error) {
	results := make([]model.ProbeResult, 0, len(params))
	rows := make([]*probeResultRow, 0, len(params))
	for index := range params {
		result, row, err := s.prepareProbeResultRow(ctx, queryer, params[index])
		if err != nil {
			return nil, nil, err
		}
		results = append(results, result)
		rows = append(rows, row)
	}
	return results, rows, nil
}

func (s *resultStore) prepareProbeResultRow(
	ctx context.Context,
	queryer resultQueryer,
	params RecordProbeResultParams,
) (model.ProbeResult, *probeResultRow, error) {
	normalized, err := normalizeProbeResultParams(params)
	if err != nil {
		return model.ProbeResult{}, nil, err
	}

	monitor, err := s.monitorForAgentWithQueryer(ctx, queryer, normalized.MonitorID, normalized.Agent.ID)
	if err != nil {
		return model.ProbeResult{}, nil, err
	}

	id, err := newID("res")
	if err != nil {
		return model.ProbeResult{}, nil, err
	}
	now := time.Now().UTC()
	row := newProbeResultRow(id, normalized, monitor, now)

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
	}, row, nil
}

func (s *resultStore) monitorForAgentWithQueryer(
	ctx context.Context,
	queryer resultQueryer,
	monitorID,
	agentID string,
) (model.Monitor, error) {
	var rec monitorRecord
	err := queryer.QueryRowContext(
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
